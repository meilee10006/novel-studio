package domain

type CoreTask struct {
	TaskID        string `json:"task_id"`
	Kind          string `json:"kind"`
	Target        string `json:"target"`
	BaseCanonRoot string `json:"base_canon_root"`
}

type CoreAttempt struct {
	AttemptID         string   `json:"attempt_id"`
	Reason            string   `json:"reason"`
	ProtocolVersion   string   `json:"protocol_version"`
	TaskDigest        string   `json:"task_digest"`
	CompletionNonce   string   `json:"completion_nonce"`
	RequiredArtifacts []string `json:"required_artifacts"`
}

type CoreBlock struct {
	BlockID        string   `json:"block_id"`
	TaskID         string   `json:"task_id"`
	AttemptID      string   `json:"attempt_id"`
	BaseCanonRoot  string   `json:"base_canon_root"`
	ConstraintRefs []string `json:"constraint_refs"`
	Conflict       string   `json:"conflict"`
	Options        []string `json:"options"`
}

type CoreProductionState struct {
	SchemaVersion  int          `json:"schema_version"`
	Revision       int          `json:"revision"`
	CanonRoot      string       `json:"canon_root,omitempty"`
	ActiveTask     *CoreTask    `json:"active_task,omitempty"`
	ActiveAttempt  *CoreAttempt `json:"active_attempt,omitempty"`
	ActiveBlock    *CoreBlock   `json:"active_block,omitempty"`
	NextTaskSeq    int          `json:"next_task_seq"`
	NextAttemptSeq int          `json:"next_attempt_seq"`
	NextEntitySeq  int          `json:"next_entity_seq"`
}

type CoreIDMapping struct {
	EntityType string `json:"entity_type"`
	LocalID    string `json:"local_id"`
	CanonID    string `json:"canon_id"`
}

type CoreReceipt struct {
	SchemaVersion    int               `json:"schema_version"`
	ProjectID        string            `json:"project_id"`
	TaskID           string            `json:"task_id"`
	AttemptID        string            `json:"attempt_id"`
	PreviousRoot     string            `json:"previous_root"`
	TaskDigest       string            `json:"task_digest"`
	SubmissionDigest string            `json:"submission_digest"`
	ArtifactDigests  map[string]string `json:"artifact_digests"`
	ValidationDigest string            `json:"validation_digest"`
	Result           string            `json:"result"`
	NewRoot          string            `json:"new_root"`
	IDMappings       []CoreIDMapping   `json:"id_mappings,omitempty"`
	CommittedAt      string            `json:"committed_at"`
}

type CoreCanonState struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      int    `json:"revision"`
	ProjectID     string `json:"project_id"`
	LastTaskID    string `json:"last_task_id"`
	LastAttemptID string `json:"last_attempt_id"`
	LatestChapter int    `json:"latest_chapter,omitempty"`
}

type CoreCanonHead struct {
	SchemaVersion   int               `json:"schema_version"`
	Revision        int               `json:"revision"`
	ParentRoot      string            `json:"parent_root"`
	Root            string            `json:"root"`
	StateDigest     string            `json:"state_digest"`
	ArtifactDigests map[string]string `json:"artifact_digests"`
}
