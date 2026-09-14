package domain

// CoreProjectState is local operational metadata for the provider-free Core.
// It is authoritative for transport identity but is not part of novel canon.
type CoreProjectState struct {
	SchemaVersion   int    `json:"schema_version"`
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	WorkspaceRoot   string `json:"workspace_root"`
	CapabilityNonce string `json:"capability_nonce"`
	MarkdownProbe   string `json:"markdown_probe"`
}

type CoreMigrationReceipt struct {
	SchemaVersion   int    `json:"schema_version"`
	State           string `json:"state"`
	ProjectID       string `json:"project_id"`
	FromSchema      int    `json:"from_schema"`
	ToSchema        int    `json:"to_schema"`
	BackupPath      string `json:"backup_path"`
	CanonRootBefore string `json:"canon_root_before"`
	CanonRootAfter  string `json:"canon_root_after,omitempty"`
	CompletedAt     string `json:"completed_at,omitempty"`
}

type CoreProtocolMigrationReceipt struct {
	SchemaVersion     int          `json:"schema_version"`
	State             string       `json:"state"`
	ProjectID         string       `json:"project_id"`
	CoreSchemaVersion int          `json:"core_schema_version"`
	FromProtocol      string       `json:"from_protocol"`
	ToProtocol        string       `json:"to_protocol"`
	BackupPath        string       `json:"backup_path"`
	CanonRootBefore   string       `json:"canon_root_before"`
	CanonRootAfter    string       `json:"canon_root_after,omitempty"`
	CapabilityNonce   string       `json:"capability_nonce"`
	MarkdownProbe     string       `json:"markdown_probe"`
	NextAttempt       *CoreAttempt `json:"next_attempt,omitempty"`
	NextAttemptSeq    int          `json:"next_attempt_seq,omitempty"`
	NextBlock         *CoreBlock   `json:"next_block,omitempty"`
	CompletedAt       string       `json:"completed_at,omitempty"`
}
