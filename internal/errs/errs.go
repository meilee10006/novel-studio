// Package errs contains legacy-compatible error sentinels used by persisted store/domain code.
// Provider/model runtime classification was retired with the legacy AI runtime.
package errs

import "errors"

var (
	ErrConfig           = errors.New("config error")
	ErrProvider         = errors.New("provider error") // provider initialization / wiring
	ErrStoreRead        = errors.New("store read error")
	ErrStoreWrite       = errors.New("store write error")
	ErrToolArgs         = errors.New("tool args invalid")
	ErrToolPrecondition = errors.New("tool precondition failed")
	ErrToolConflict     = errors.New("tool conflict")
	ErrPhaseTransition  = errors.New("invalid phase transition")
	ErrFlowTransition   = errors.New("invalid flow transition")
)
