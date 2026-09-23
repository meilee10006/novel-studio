package protocol

type DesignManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ProjectID       string   `json:"project_id"`
	SubmissionID    string   `json:"submission_id"`
	ProtocolVersion string   `json:"protocol_version"`
	Operation       string   `json:"operation"`
	Files           []string `json:"files"`
}
