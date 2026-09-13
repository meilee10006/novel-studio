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
