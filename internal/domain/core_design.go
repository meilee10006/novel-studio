package domain

const (
	DesignModeRequired = "required"
	DesignModeLegacy   = "legacy"

	DesignCheckpointStoryLocked     = "story_locked"
	DesignCheckpointFoundationReady = "foundation_ready"
)

type CoreDesignArtifact struct {
	SchemaVersion int      `json:"schema_version"`
	ArtifactType  string   `json:"artifact_type"`
	Inputs        []string `json:"inputs,omitempty"`
	Sources       []string `json:"sources,omitempty"`
	Supersedes    string   `json:"supersedes,omitempty"`
	Payload       any      `json:"payload"`
}

type CoreDesignBundle struct {
	SchemaVersion int               `json:"schema_version"`
	Selections    map[string]string `json:"selections"`
}

type CoreDesignCommit struct {
	SchemaVersion           int               `json:"schema_version"`
	Checkpoint              string            `json:"checkpoint"`
	ParentDesignRoot        string            `json:"parent_design_root,omitempty"`
	BundleRef               string            `json:"bundle_ref"`
	Evidence                map[string]string `json:"evidence"`
	CheckpointPolicyVersion int               `json:"checkpoint_policy_version"`
}

type CoreDesignHead struct {
	SchemaVersion int    `json:"schema_version"`
	DesignRoot    string `json:"design_root"`
	Checkpoint    string `json:"checkpoint"`
}

type CoreDesignReceipt struct {
	SchemaVersion      int    `json:"schema_version"`
	SubmissionID       string `json:"submission_id"`
	Operation          string `json:"operation"`
	Result             string `json:"result"`
	SnapshotDigest     string `json:"snapshot_digest"`
	PreviousDesignRoot string `json:"previous_design_root,omitempty"`
	NewDesignRoot      string `json:"new_design_root,omitempty"`
	Problem            string `json:"problem,omitempty"`
	CommittedAt        string `json:"committed_at,omitempty"`
}

type CoreDesignSubmissionRecord struct {
	SchemaVersion      int               `json:"schema_version"`
	SubmissionID       string            `json:"submission_id"`
	Operation          string            `json:"operation,omitempty"`
	State              string            `json:"state"`
	ObservedDigest     string            `json:"observed_digest,omitempty"`
	ObservedAt         string            `json:"observed_at,omitempty"`
	SnapshotDigest     string            `json:"snapshot_digest,omitempty"`
	Conflict           bool              `json:"conflict,omitempty"`
	Problem            string            `json:"problem,omitempty"`
	Result             string            `json:"result,omitempty"`
	Refs               map[string]string `json:"refs,omitempty"`
	PreviousDesignRoot string            `json:"previous_design_root,omitempty"`
	NewDesignRoot      string            `json:"new_design_root,omitempty"`
	ReceiptPath        string            `json:"receipt_path,omitempty"`
}

type CoreDesignSubmissionResult struct {
	SchemaVersion      int               `json:"schema_version"`
	SubmissionID       string            `json:"submission_id"`
	Result             string            `json:"result"`
	Refs               map[string]string `json:"refs,omitempty"`
	PreviousDesignRoot string            `json:"previous_design_root,omitempty"`
	NewDesignRoot      string            `json:"new_design_root,omitempty"`
	ReceiptRef         string            `json:"receipt_ref,omitempty"`
	Problem            string            `json:"problem,omitempty"`
}
