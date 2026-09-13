package domain

// CoreCommitJournal is the recoverable local transaction plan for one accepted attempt.
type CoreCommitJournal struct {
	SchemaVersion int                 `json:"schema_version"`
	State         string              `json:"state"`
	TaskID        string              `json:"task_id"`
	AttemptID     string              `json:"attempt_id"`
	Chapter       int                 `json:"chapter"`
	PreviousRoot  string              `json:"previous_root"`
	NewRoot       string              `json:"new_root"`
	ArtifactNames []string            `json:"artifact_names"`
	CanonState    CoreCanonState      `json:"canon_state"`
	CanonHead     CoreCanonHead       `json:"canon_head"`
	Receipt       CoreReceipt         `json:"receipt"`
	NextState     CoreProductionState `json:"next_state"`
}
