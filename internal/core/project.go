package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"github.com/chenhongyang/novel-studio/internal/store"
)

type Project struct {
	root                  string
	store                 *store.Store
	submissionQuietPeriod time.Duration
	commitFault           func(string) error
	controlFault          func(string) error
}

type Status struct {
	Root              string       `json:"root"`
	Initialized       bool         `json:"initialized"`
	NovelName         string       `json:"novel_name,omitempty"`
	Phase             domain.Phase `json:"phase,omitempty"`
	CurrentChapter    int          `json:"current_chapter,omitempty"`
	TotalChapters     int          `json:"total_chapters,omitempty"`
	CompletedChapters int          `json:"completed_chapters,omitempty"`
	Warnings          []string     `json:"warnings,omitempty"`
	ProjectID         string       `json:"project_id,omitempty"`
	ProtocolVersion   string       `json:"protocol_version,omitempty"`
	Capability        string       `json:"capability,omitempty"`
	CapabilityProblem string       `json:"capability_problem,omitempty"`
	CanonRoot         string       `json:"canon_root,omitempty"`
	ActiveTaskKind    string       `json:"active_task_kind,omitempty"`
	ActiveTarget      string       `json:"active_target,omitempty"`
	ActiveAttemptID   string       `json:"active_attempt_id,omitempty"`
	BlockID           string       `json:"block_id,omitempty"`
}

type Verification struct {
	Root     string   `json:"root"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems,omitempty"`
}

func OpenProject(root string) (*Project, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("project root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open project %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root %q is not a directory", abs)
	}
	return &Project{root: abs, store: store.NewStore(abs), submissionQuietPeriod: 250 * time.Millisecond}, nil
}

func (p *Project) Status() (Status, error) {
	progress, err := p.store.Progress.Load()
	if err != nil {
		return Status{}, fmt.Errorf("load progress: %w", err)
	}
	out := Status{Root: p.root, Warnings: p.store.CheckConsistency()}
	state, err := p.store.LoadCoreProjectState()
	if err != nil {
		return Status{}, fmt.Errorf("load core project metadata: %w", err)
	}
	if state != nil {
		out.Initialized = true
		out.ProjectID = state.ProjectID
		out.ProtocolVersion = state.ProtocolVersion
		out.Capability, out.CapabilityProblem = capabilityStatus(state)
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return Status{}, fmt.Errorf("load core production state: %w", err)
	}
	if production != nil {
		out.CanonRoot = production.CanonRoot
		if production.ActiveTask != nil {
			out.ActiveTaskKind = production.ActiveTask.Kind
			out.ActiveTarget = production.ActiveTask.Target
		}
		if production.ActiveAttempt != nil {
			out.ActiveAttemptID = production.ActiveAttempt.AttemptID
		}
		if production.ActiveBlock != nil {
			out.BlockID = production.ActiveBlock.BlockID
		}
	}
	if progress == nil {
		return out, nil
	}
	out.Initialized = true
	out.NovelName = progress.NovelName
	out.Phase = progress.Phase
	out.CurrentChapter = progress.CurrentChapter
	out.TotalChapters = progress.TotalChapters
	out.CompletedChapters = len(progress.CompletedChapters)
	return out, nil
}

func (p *Project) Verify() (Verification, error) {
	release, err := p.acquireProjectReadLock()
	if err != nil {
		return Verification{}, err
	}
	defer release()
	status, err := p.Status()
	if err != nil {
		return Verification{}, err
	}
	problems := append([]string(nil), status.Warnings...)
	if status.Capability == "invalid" {
		problems = append(problems, "capability check: "+status.CapabilityProblem)
	}
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		problems = append(problems, "canon head: "+err.Error())
	} else if head != nil {
		recomputed, err := p.RecomputeCanonRoot()
		if err != nil {
			problems = append(problems, "canon root: "+err.Error())
		} else if recomputed != head.Root {
			problems = append(problems, fmt.Sprintf("canon root mismatch: recomputed %s, recorded %s", recomputed, head.Root))
		}
		problems = append(problems, p.verifyReceiptChain(head)...)
	}
	return Verification{Root: p.root, OK: len(problems) == 0, Problems: problems}, nil
}

func (p *Project) verifyReceiptChain(head *domain.CoreCanonHead) []string {
	var problems []string
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		problems = append(problems, "receipt chain production state: "+err.Error())
	} else if production == nil {
		problems = append(problems, "receipt chain production state is missing")
	} else if production.CanonRoot != head.Root {
		problems = append(problems, fmt.Sprintf("receipt chain active root mismatch: production %s, head %s", production.CanonRoot, head.Root))
	}
	receipts, err := p.store.ListCoreReceipts()
	if err != nil {
		return append(problems, "receipt chain: "+err.Error())
	}
	byRoot := map[string][]domain.CoreReceipt{}
	for _, receipt := range receipts {
		if receipt.Result != "ACCEPTED" || receipt.NewRoot == "" {
			continue
		}
		byRoot[receipt.NewRoot] = append(byRoot[receipt.NewRoot], receipt)
	}
	seen := map[string]bool{}
	current := head.Root
	first := true
	for current != "" {
		if seen[current] {
			problems = append(problems, "receipt chain contains a cycle at "+current)
			break
		}
		seen[current] = true
		candidates := byRoot[current]
		if len(candidates) != 1 {
			problems = append(problems, fmt.Sprintf("receipt chain root %s has %d accepted receipts", current, len(candidates)))
			break
		}
		receipt := candidates[0]
		if first && receipt.PreviousRoot != head.ParentRoot {
			problems = append(problems, fmt.Sprintf("receipt chain head parent mismatch: receipt %s, head %s", receipt.PreviousRoot, head.ParentRoot))
		}
		current = receipt.PreviousRoot
		first = false
	}
	return problems
}

type capabilityAck struct {
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	Nonce           string `json:"nonce"`
	Capabilities    struct {
		Read          bool `json:"read"`
		WriteUTF8JSON bool `json:"write_utf8_json"`
		WriteUTF8MD   bool `json:"write_utf8_md"`
	} `json:"capabilities"`
}

func capabilityStatus(state *domain.CoreProjectState) (string, string) {
	var ack capabilityAck
	err := protocol.ReadJSON(state.WorkspaceRoot, "setup/capability-ack.json", 64<<10, &ack)
	if errors.Is(err, os.ErrNotExist) {
		return "pending", ""
	}
	if err != nil {
		return "invalid", err.Error()
	}
	if ack.ProjectID != state.ProjectID || ack.ProtocolVersion != state.ProtocolVersion || ack.Nonce != state.CapabilityNonce {
		return "invalid", "capability acknowledgement identity or nonce mismatch"
	}
	if !ack.Capabilities.Read || !ack.Capabilities.WriteUTF8JSON || !ack.Capabilities.WriteUTF8MD {
		return "invalid", "required plain-file capabilities were not acknowledged"
	}
	probe, err := protocol.ReadUTF8(state.WorkspaceRoot, "setup/capability-write-test.md", 64<<10)
	if errors.Is(err, os.ErrNotExist) {
		return "pending", ""
	}
	if err != nil {
		return "invalid", err.Error()
	}
	if string(probe) != state.MarkdownProbe {
		return "invalid", "capability markdown probe mismatch"
	}
	return "passed", ""
}
