package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	LegacyVersion      = "0.9"
	CurrentVersion     = "1.0"
	MaxJSONDepth       = 64
	MaxJSONArrayLength = 10000
	DefaultMaxTextSize = 8 << 20
)

func IsKnownVersion(version string) bool {
	return version == LegacyVersion || version == CurrentVersion
}

func RenderChatGPTProtocol(projectID string) string {
	return fmt.Sprintf(`# Novel Core × ChatGPT App 协议

协议版本：%s
项目：%s

## 基本规则

- 只处理普通 UTF-8 .json、.md、.txt 文件。
- Core 写 project.json、CHATGPT_PROTOCOL.md、READY、STATUS、outbox、result、published、projection、backup。
- ChatGPT 只写 exchange/inbox、exchange/control/inbox，以及能力检查要求的 setup 回执文件。
- 不修改 Core 写入的文件，不使用 Google Docs/Sheets 代替协议文件。
- 正式提交时先写全部 artifact，最后写 manifest.json。
- 当前任务只以 exchange/READY.json 为准；没有 READY 时不要自行推进小说状态。

## 能力检查

首次初始化后读取 setup/capability-challenge.json，按其中 nonce 和 probe 写回：

- setup/capability-ack.json
- setup/capability-write-test.md

能力检查通过前不会生成正式 READY。
`, CurrentVersion, projectID)
}

func DecodeJSON(data []byte, dst any) error {
	var shape any
	if err := json.Unmarshal(data, &shape); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := validateJSONShape(shape, 1); err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode json target: %w", err)
	}
	return nil
}

func validateJSONShape(v any, depth int) error {
	if depth > MaxJSONDepth {
		return fmt.Errorf("json exceeds max depth %d", MaxJSONDepth)
	}
	switch x := v.(type) {
	case []any:
		if len(x) > MaxJSONArrayLength {
			return fmt.Errorf("json array exceeds max length %d", MaxJSONArrayLength)
		}
		for _, item := range x {
			if err := validateJSONShape(item, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range x {
			if err := validateJSONShape(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

const MachineSchemaVersion = 1

type SubmissionManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ProjectID       string   `json:"project_id"`
	TaskID          string   `json:"task_id"`
	AttemptID       string   `json:"attempt_id"`
	BaseCanonRoot   string   `json:"base_canon_root"`
	ProtocolVersion string   `json:"protocol_version"`
	TaskDigest      string   `json:"task_digest"`
	CompletionNonce string   `json:"completion_nonce"`
	Files           []string `json:"files"`
}
