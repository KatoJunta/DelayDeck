package state

import "errors"

var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrTransitionPending = errors.New("transition already in progress")
)

type TransitionError struct {
	From    State
	To      State
	Trigger string
}

func (e *TransitionError) Error() string {
	return "invalid transition from " + string(e.From) + " via " + e.Trigger
}

func (e *TransitionError) Is(target error) bool {
	return target == ErrInvalidTransition
}
