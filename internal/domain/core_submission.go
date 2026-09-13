package domain

// CoreSubmissionRecord records reconciliation state for one immutable attempt.
type CoreSubmissionRecord struct {
	SchemaVersion  int    `json:"schema_version"`
	TaskID         string `json:"task_id"`
	AttemptID      string `json:"attempt_id"`
	State          string `json:"state"`
	ObservedDigest string `json:"observed_digest,omitempty"`
	ObservedAt     string `json:"observed_at,omitempty"`
	SnapshotDigest string `json:"snapshot_digest,omitempty"`
	Conflict       bool   `json:"conflict,omitempty"`
	Problem        string `json:"problem,omitempty"`
}
