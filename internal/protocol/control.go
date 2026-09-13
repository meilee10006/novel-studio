package protocol

// ControlManifest declares a complete control message payload.
type ControlManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ProjectID       string   `json:"project_id"`
	MessageID       string   `json:"message_id"`
	BaseCanonRoot   string   `json:"base_canon_root"`
	ProtocolVersion string   `json:"protocol_version"`
	Files           []string `json:"files"`
}
