package domain

// Stage is where a model version is in its lifecycle.
type Stage string

// The stages, in the order a successful version usually moves through them.
const (
	StageRegistered Stage = "registered"
	StageValidated  Stage = "validated"
	StageShadow     Stage = "shadow"
	StageProduction Stage = "production"
	StageRetired    Stage = "retired"
	StageRejected   Stage = "rejected"
	StageRolledBack Stage = "rolled_back"
)

// transitions is the complete list of allowed stage changes. Anything not listed is illegal.
var transitions = map[Stage][]Stage{
	StageRegistered: {StageValidated, StageRejected},
	StageValidated:  {StageShadow, StageRejected},
	StageShadow:     {StageProduction, StageRejected},
	StageProduction: {StageRetired, StageRolledBack},
	StageRetired:    {StageProduction}, // restore the previous version during a rollback
	StageRejected:   {},                // terminal
	StageRolledBack: {},                // terminal
}

// Stages returns every stage.
func Stages() []Stage {
	return []Stage{StageRegistered, StageValidated, StageShadow, StageProduction, StageRetired, StageRejected, StageRolledBack}
}

// Valid reports whether s is a known stage.
func (s Stage) Valid() bool {
	_, ok := transitions[s]
	return ok
}

// CanTransition reports whether a version may move from one stage to another.
func CanTransition(from, to Stage) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// AllowedFrom returns the stages a version in stage s may move to.
func AllowedFrom(s Stage) []Stage {
	return append([]Stage(nil), transitions[s]...)
}
