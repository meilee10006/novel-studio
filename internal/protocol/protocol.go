package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
)

const (
	LegacyVersion      = "0.9"
	PreviousVersion    = "1.0"
	CurrentVersion     = "1.1"
	MaxJSONDepth       = 64
	MaxJSONArrayLength = 10000
	DefaultMaxTextSize = 8 << 20
)

func IsKnownVersion(version string) bool {
	return version == LegacyVersion || version == PreviousVersion || version == CurrentVersion
}

func DecodeJSON(data []byte, dst any) error {
	if err := validateJSONTokenStream(data); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
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

func validateJSONTokenStream(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		if number, ok := token.(json.Number); ok {
			return rejectLossyIntegerLiteral(number)
		}
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			seen[key] = true
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("unterminated JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func rejectLossyIntegerLiteral(number json.Number) error {
	raw := number.String()
	exact, ok := new(big.Rat).SetString(raw)
	if !ok {
		return fmt.Errorf("invalid JSON number %q", raw)
	}
	if !exact.IsInt() {
		return nil
	}
	asFloat, _ := new(big.Float).SetInt(exact.Num()).Float64()
	restored, _ := new(big.Float).SetFloat64(asFloat).Int(nil)
	if restored == nil || restored.Cmp(exact.Num()) != 0 {
		return fmt.Errorf("JSON integer %s cannot be represented exactly", raw)
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
