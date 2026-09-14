package domain

type CoreEventEvidence struct {
	EventID   string   `json:"event_id"`
	Chapter   int      `json:"chapter"`
	Observers []string `json:"observers,omitempty"`
	Actors    []string `json:"actors,omitempty"`
}

type CoreEntityState struct {
	EntityType      string `json:"entity_type"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	EvidenceEventID string `json:"evidence_event_id,omitempty"`
}

type CoreKnowledgeFact struct {
	Statement       string `json:"statement,omitempty"`
	SourceKind      string `json:"source_kind"`
	EvidenceEventID string `json:"evidence_event_id"`
	FromCharacterID string `json:"from_character_id,omitempty"`
}

type CoreLocationState struct {
	LocationID      string `json:"location_id"`
	StartTick       int64  `json:"start_tick"`
	EndTick         int64  `json:"end_tick"`
	EvidenceEventID string `json:"evidence_event_id"`
}

type CoreEvidenceState struct {
	Tags            []string `json:"tags,omitempty"`
	EvidenceEventID string   `json:"evidence_event_id"`
}
type CoreForeshadowState struct {
	Description     string `json:"description,omitempty"`
	State           string `json:"state"`
	EvidenceEventID string `json:"evidence_event_id,omitempty"`
}

type CoreConflictState struct {
	Description         string   `json:"description,omitempty"`
	Participants        []string `json:"participants,omitempty"`
	State               string   `json:"state"`
	EscalationCondition string   `json:"escalation_condition,omitempty"`
	CloseCondition      string   `json:"close_condition,omitempty"`
	EvidenceEventID     string   `json:"evidence_event_id,omitempty"`
}

type CoreReaderPromiseState struct {
	Statement       string `json:"statement,omitempty"`
	State           string `json:"state"`
	EvidenceEventID string `json:"evidence_event_id,omitempty"`
	DeadlineChapter int    `json:"deadline_chapter,omitempty"`
}

type CoreTravelConstraint struct {
	FromLocationID string `json:"from_location_id"`
	ToLocationID   string `json:"to_location_id"`
	MinTicks       int64  `json:"min_ticks"`
}

type CoreEndingState struct {
	MainResolutionEventID string `json:"main_resolution_event_id"`
}

type CoreLongformState struct {
	Entities          map[string]CoreEntityState              `json:"entities,omitempty"`
	Events            map[string]CoreEventEvidence            `json:"events,omitempty"`
	Knowledge         map[string]map[string]CoreKnowledgeFact `json:"knowledge,omitempty"`
	Locations         map[string]CoreLocationState            `json:"locations,omitempty"`
	Resources         map[string]int64                        `json:"resources,omitempty"`
	Relationships     map[string]CoreEvidenceState            `json:"relationships,omitempty"`
	Foreshadows       map[string]CoreForeshadowState          `json:"foreshadows,omitempty"`
	Conflicts         map[string]CoreConflictState            `json:"conflicts,omitempty"`
	ReaderPromises    map[string]CoreReaderPromiseState       `json:"reader_promises,omitempty"`
	TravelConstraints []CoreTravelConstraint                  `json:"travel_constraints,omitempty"`
	Ending            *CoreEndingState                        `json:"ending,omitempty"`
}
