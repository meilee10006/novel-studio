package store

import (
	"os"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

const coreProjectStatePath = "meta/core/project.json"

func (s *Store) LoadCoreProjectState() (*domain.CoreProjectState, error) {
	var state domain.CoreProjectState
	if err := s.Progress.io.ReadJSON(coreProjectStatePath, &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *Store) SaveCoreProjectState(state *domain.CoreProjectState) error {
	if state == nil {
		return nil
	}
	return s.Progress.io.WriteJSON(coreProjectStatePath, state)
}
