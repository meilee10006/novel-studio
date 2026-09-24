package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"github.com/chenhongyang/novel-studio/internal/store"
)

const coreSchemaVersion = 2

var projectIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type InitOptions struct {
	ProjectID     string
	LocalRoot     string
	WorkspaceRoot string
}

type workspaceProject struct {
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	Mode            string `json:"mode"`
}

type capabilityChallenge struct {
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	Nonce           string `json:"nonce"`
	AckPath         string `json:"ack_path"`
	MarkdownPath    string `json:"markdown_path"`
	MarkdownProbe   string `json:"markdown_probe"`
}

func InitProject(opts InitOptions) (*Project, error) {
	opts.ProjectID = strings.TrimSpace(opts.ProjectID)
	if !projectIDPattern.MatchString(opts.ProjectID) {
		return nil, fmt.Errorf("invalid project id %q", opts.ProjectID)
	}
	if strings.TrimSpace(opts.LocalRoot) == "" || strings.TrimSpace(opts.WorkspaceRoot) == "" {
		return nil, fmt.Errorf("local project root and Drive workspace are required")
	}
	local, err := filepath.Abs(opts.LocalRoot)
	if err != nil {
		return nil, err
	}
	workspace, err := filepath.Abs(opts.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	if pathsOverlap(local, workspace) {
		return nil, fmt.Errorf("local authority and Drive workspace must be separate")
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		return nil, fmt.Errorf("create local project root: %w", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create Drive workspace: %w", err)
	}
	localResolved, err := filepath.EvalSymlinks(local)
	if err != nil {
		return nil, err
	}
	workspaceResolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, err
	}
	if pathsOverlap(localResolved, workspaceResolved) {
		return nil, fmt.Errorf("local authority and Drive workspace resolve to overlapping paths")
	}

	st := store.NewCoreStore(localResolved)
	state, err := st.LoadCoreProjectState()
	if err != nil {
		return nil, fmt.Errorf("load local project metadata: %w", err)
	}
	if state == nil {
		nonce, err := randomNonce()
		if err != nil {
			return nil, err
		}
		state = &domain.CoreProjectState{
			SchemaVersion:   coreSchemaVersion,
			ProjectID:       opts.ProjectID,
			ProtocolVersion: protocol.CurrentVersion,
			WorkspaceRoot:   workspaceResolved,
			CapabilityNonce: nonce,
			MarkdownProbe:   "novel-core-capability:" + nonce + "\n",
			DesignMode:      domain.DesignModeRequired,
		}
		if err := st.SaveCoreProjectState(state); err != nil {
			return nil, fmt.Errorf("save local project metadata: %w", err)
		}
	} else {
		if state.ProjectID != opts.ProjectID || state.ProtocolVersion != protocol.CurrentVersion || state.WorkspaceRoot != workspaceResolved {
			return nil, fmt.Errorf("existing local project metadata does not match init arguments")
		}
	}

	for _, dir := range []string{
		"setup", "exchange/outbox", "exchange/inbox", "exchange/result",
		"exchange/control/inbox", "exchange/control/result",
		"exchange/design/inbox", "exchange/design/result",
		"published", "projection", "backup",
	} {
		if err := os.MkdirAll(filepath.Join(workspaceResolved, dir), 0o755); err != nil {
			return nil, err
		}
	}
	if err := writeWorkspaceJSON(workspaceResolved, "project.json", workspaceProject{
		ProjectID: opts.ProjectID, ProtocolVersion: protocol.CurrentVersion, Mode: "chatgpt_app_drive",
	}); err != nil {
		return nil, err
	}
	if err := protocol.WriteUTF8Atomic(workspaceResolved, "CHATGPT_PROTOCOL.md", []byte(protocol.RenderChatGPTProtocol(opts.ProjectID)), 0o644); err != nil {
		return nil, err
	}
	challenge := capabilityChallenge{
		ProjectID: opts.ProjectID, ProtocolVersion: protocol.CurrentVersion, Nonce: state.CapabilityNonce,
		AckPath: "setup/capability-ack.json", MarkdownPath: "setup/capability-write-test.md", MarkdownProbe: state.MarkdownProbe,
	}
	if err := writeWorkspaceJSON(workspaceResolved, "setup/capability-challenge.json", challenge); err != nil {
		return nil, err
	}
	project, err := OpenProject(localResolved)
	if err != nil {
		return nil, err
	}
	production, err := st.LoadCoreProductionState()
	if err != nil {
		return nil, err
	}
	if err := project.writeWorkspaceStatus(state, production); err != nil {
		return nil, err
	}
	return project, nil
}

func writeWorkspaceJSON(root, rel string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return protocol.WriteUTF8Atomic(root, rel, data, 0o644)
}

func randomNonce() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate capability nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func pathsOverlap(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	return isWithin(a, b) || isWithin(b, a)
}

func isWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
