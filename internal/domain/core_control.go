package domain

// CoreControlRecord tracks reconciliation and settlement for one control message.
type CoreControlRecord struct {
	SchemaVersion  int                  `json:"schema_version"`
	MessageID      string               `json:"message_id"`
	State          string               `json:"state"`
	ObservedDigest string               `json:"observed_digest,omitempty"`
	ObservedAt     string               `json:"observed_at,omitempty"`
	SnapshotDigest string               `json:"snapshot_digest,omitempty"`
	Conflict       bool                 `json:"conflict,omitempty"`
	Problem        string               `json:"problem,omitempty"`
	Result         string               `json:"result,omitempty"`
	TaskID         string               `json:"task_id,omitempty"`
	AttemptID      string               `json:"attempt_id,omitempty"`
	NextProduction *CoreProductionState `json:"next_production,omitempty"`
}

type CoreControlMessage struct {
	SchemaVersion  int    `json:"schema_version"`
	ProjectID      string `json:"project_id"`
	MessageID      string `json:"message_id"`
	Kind           string `json:"kind"`
	BaseCanonRoot  string `json:"base_canon_root"`
	BlockID        string `json:"block_id,omitempty"`
	Choice         string `json:"choice,omitempty"`
	DirectiveScope string `json:"directive_scope,omitempty"`
	Instruction    string `json:"instruction,omitempty"`
	Chapter        int    `json:"chapter,omitempty"`
}
