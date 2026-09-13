package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// SessionStore 追加式记录旧 AI runtime 的对话历史。它只处理 JSON 形状，
// 不依赖任何模型或 agent 类型，因此 provider-free Core 可以安全复用 Store。
type SessionStore struct {
	io      *IO
	mu      sync.Mutex
	seq     map[string]int
	taskKey map[string]string
}

func NewSessionStore(io *IO) *SessionStore {
	return &SessionStore{io: io, seq: make(map[string]int), taskKey: make(map[string]string)}
}

type ModelLookup func(agentName string) (provider, model string)

func (s *SessionStore) CoordinatorLogger(lookup ModelLookup) func(any) {
	return func(msg any) {
		var meta *sessionLogMeta
		if lookup != nil {
			meta = lookupMeta(lookup, "coordinator")
		}
		if err := s.logEntry("meta/sessions/coordinator.jsonl", msg, meta); err != nil {
			slog.Warn("session log failed", "agent", "coordinator", "err", err)
		}
	}
}

func (s *SessionStore) SubAgentLogger(lookup ModelLookup) func(agentName, task string, msg any) {
	return func(agentName, task string, msg any) {
		var meta *sessionLogMeta
		if lookup != nil {
			meta = lookupMeta(lookup, agentName)
		}
		if err := s.logEntry(s.subAgentPath(agentName, task), msg, meta); err != nil {
			slog.Warn("session log failed", "agent", agentName, "err", err)
		}
	}
}

func lookupMeta(lookup ModelLookup, agentName string) *sessionLogMeta {
	provider, model := lookup(agentName)
	if provider == "" && model == "" {
		return nil
	}
	return &sessionLogMeta{Provider: provider, Model: model}
}

func (s *SessionStore) LogCoCreate(entry any) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cocreate session: %w", err)
	}
	return s.io.AppendLine("meta/sessions/cocreate.jsonl", append(data, '\n'))
}

func (s *SessionStore) Log(rel string, msg any) error { return s.logEntry(rel, msg, nil) }

type sessionLogMeta struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

func (s *SessionStore) logEntry(rel string, msg any, fallback *sessionLogMeta) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal session message: %w", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var role string
	if err := json.Unmarshal(obj["role"], &role); err != nil || role == "" {
		return nil
	}
	// 旧实现只接受具体的 agentcore.Message。去掉类型依赖后仍用其稳定 JSON
	// 形状做边界：自定义状态消息即使碰巧有 role，也不应写入会话日志。
	if _, ok := obj["content"]; !ok {
		return nil
	}
	if _, ok := obj["timestamp"]; !ok {
		return nil
	}

	compactMessageJSON(obj, role)
	if role == "assistant" && rawPresent(obj["usage"]) {
		meta := usageMetaJSON(obj["usage"])
		if meta == nil {
			meta = fallback
		}
		if meta != nil {
			encoded, err := json.Marshal(meta)
			if err != nil {
				return err
			}
			obj["_meta"] = encoded
		}
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal session entry: %w", err)
	}
	return s.io.AppendLine(rel, append(data, '\n'))
}

func rawPresent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func usageMetaJSON(raw json.RawMessage) *sessionLogMeta {
	var usage struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if json.Unmarshal(raw, &usage) != nil || (usage.Provider == "" && usage.Model == "") {
		return nil
	}
	return &sessionLogMeta{Provider: usage.Provider, Model: usage.Model}
}

func (s *SessionStore) subAgentPath(agentName, task string) string {
	suffix := extractChapter(task)
	if suffix != "" {
		return fmt.Sprintf("meta/sessions/agents/%s-%s.jsonl", agentName, suffix)
	}
	key := agentName + "|" + task
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.taskKey[key]; ok {
		return fmt.Sprintf("meta/sessions/agents/%s-%s.jsonl", agentName, cached)
	}
	s.seq[agentName]++
	suffix = fmt.Sprintf("%03d", s.seq[agentName])
	s.taskKey[key] = suffix
	return fmt.Sprintf("meta/sessions/agents/%s-%s.jsonl", agentName, suffix)
}

var chapterRe = regexp.MustCompile(`第\s*(\d+)\s*章`)

func extractChapter(task string) string {
	m := chapterRe.FindStringSubmatch(task)
	if len(m) < 2 {
		return ""
	}
	n, _ := strconv.Atoi(m[1])
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("ch%02d", n)
}

func compactMessageJSON(obj map[string]json.RawMessage, role string) {
	raw := obj["content"]
	if !rawPresent(raw) {
		return
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return
	}
	toolName := toolNameFromMetaJSON(obj["metadata"])
	changed := false
	for _, block := range blocks {
		var kind string
		_ = json.Unmarshal(block["type"], &kind)
		switch kind {
		case "text":
			var text string
			if json.Unmarshal(block["text"], &text) == nil {
				compacted := compactText(role, toolName, text)
				if compacted != text {
					block["text"], _ = json.Marshal(compacted)
					changed = true
				}
			}
		case "tool_call":
			if compactToolCallJSON(block) {
				changed = true
			}
		}
	}
	if changed {
		obj["content"], _ = json.Marshal(blocks)
	}
}

func toolNameFromMetaJSON(raw json.RawMessage) string {
	if !rawPresent(raw) {
		return ""
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(raw, &meta) != nil {
		return ""
	}
	var name string
	_ = json.Unmarshal(meta["tool_name"], &name)
	return name
}

func compactText(role, toolName, text string) string {
	if role != "tool" || len(text) < 4096 {
		return text
	}
	switch toolName {
	case "novel_context":
		return fmt.Sprintf("[session_compact: novel_context %dB | %s]", len(text), extractJSONField(text, "_loading_summary"))
	case "read_chapter":
		return fmt.Sprintf("[session_compact: read_chapter %d字 | 见 chapters/]", utf8.RuneCountInString(text))
	default:
		if len(text) > 8192 {
			return fmt.Sprintf("[session_compact: %s %d字]", toolName, utf8.RuneCountInString(text))
		}
		return text
	}
}

func compactToolCallJSON(block map[string]json.RawMessage) bool {
	var tc map[string]json.RawMessage
	if json.Unmarshal(block["tool_call"], &tc) != nil {
		return false
	}
	var name string
	_ = json.Unmarshal(tc["name"], &name)
	var label, ref string
	switch name {
	case "draft_chapter":
		label, ref = "第N章正文", "drafts/"
	case "save_foundation":
		label, ref = "foundation", "store"
	default:
		return false
	}
	argsRaw := tc["args"]
	var args map[string]json.RawMessage
	if json.Unmarshal(argsRaw, &args) != nil {
		return false
	}
	contentRaw, ok := args["content"]
	if !ok || len(contentRaw) < 4096 {
		return false
	}

	if name == "save_foundation" {
		var t string
		if json.Unmarshal(args["type"], &t) == nil && t != "" {
			label = t
		}
		args["content"], _ = json.Marshal(fmt.Sprintf("[session_compact: %s %dB | 见 store]", label, len(contentRaw)))
	} else {
		var content string
		if json.Unmarshal(contentRaw, &content) != nil {
			args["content"], _ = json.Marshal(fmt.Sprintf("[session_compact: %s %dB | 见 %s]", label, len(contentRaw), ref))
		} else {
			ch := extractJSONFieldInt(argsRaw, "chapter")
			if ch > 0 {
				label, ref = fmt.Sprintf("第%d章正文", ch), fmt.Sprintf("drafts/%02d.draft.md", ch)
			}
			args["content"], _ = json.Marshal(fmt.Sprintf("[session_compact: %s %d字 | 见 %s]", label, utf8.RuneCountInString(content), ref))
		}
	}
	tc["args"], _ = json.Marshal(args)
	block["tool_call"], _ = json.Marshal(tc)
	return true
}

func extractJSONField(jsonStr, field string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(jsonStr), &m) != nil {
		return ""
	}
	raw, ok := m[field]
	if !ok {
		return ""
	}
	var val string
	if json.Unmarshal(raw, &val) != nil {
		return string(raw)
	}
	return val
}

func extractJSONFieldInt(data json.RawMessage, field string) int {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return 0
	}
	raw, ok := m[field]
	if !ok {
		return 0
	}
	var val int
	if json.Unmarshal(raw, &val) != nil {
		return 0
	}
	return val
}

const CompactTag = "[session_compact:"
