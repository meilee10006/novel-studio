package domain

type CoreEventEvidence struct {
	EventID   string   `json:"event_id"`
	Chapter   int      `json:"chapter"`
	Observers []string `json:"observers,omitempty"`
	Actors    []string `json:"actors,omitempty"`
}

type CoreKnowledgeFact struct {
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
	State           string `json:"state"`
	EvidenceEventID string `json:"evidence_event_id,omitempty"`
}

type CoreReaderPromiseState struct {
	State           string `json:"state"`
	EvidenceEventID string `json:"evidence_event_id,omitempty"`
	DeadlineChapter int    `json:"deadline_chapter,omitempty"`
}

type CoreTravelConstraint struct {
	FromLocationID string `json:"from_location_id"`
	ToLocationID   string `json:"to_location_id"`
	MinTicks       int64  `json:"min_ticks"`
}

type CoreLongformState struct {
	Events            map[string]CoreEventEvidence            `json:"events,omitempty"`
	Knowledge         map[string]map[string]CoreKnowledgeFact `json:"knowledge,omitempty"`
	Locations         map[string]CoreLocationState            `json:"locations,omitempty"`
	Resources         map[string]int64                        `json:"resources,omitempty"`
	Relationships     map[string]CoreEvidenceState            `json:"relationships,omitempty"`
	Foreshadows       map[string]CoreForeshadowState          `json:"foreshadows,omitempty"`
	ReaderPromises    map[string]CoreReaderPromiseState       `json:"reader_promises,omitempty"`
	TravelConstraints []CoreTravelConstraint                  `json:"travel_constraints,omitempty"`
}
