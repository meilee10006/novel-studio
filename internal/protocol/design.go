package protocol

type DesignManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ProjectID       string   `json:"project_id"`
	SubmissionID    string   `json:"submission_id"`
	ProtocolVersion string   `json:"protocol_version"`
	Operation       string   `json:"operation"`
	Files           []string `json:"files"`
}

type DesignPromoteRequest struct {
	SchemaVersion      int               `json:"schema_version"`
	ProjectID          string            `json:"project_id"`
	SubmissionID       string            `json:"submission_id"`
	ProtocolVersion    string            `json:"protocol_version"`
	ExpectedDesignRoot string            `json:"expected_design_root,omitempty"`
	Checkpoint         string            `json:"checkpoint"`
	BundleRef          string            `json:"bundle_ref"`
	Evidence           map[string]string `json:"evidence"`
}
