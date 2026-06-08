package state

import "fmt"

type State string

const (
	Stopped              State = "STOPPED"
	Starting             State = "STARTING"
	Ready                State = "READY"
	Ingesting            State = "INGESTING"
	Realtime             State = "REALTIME"
	BufferingToDelay     State = "BUFFERING_TO_DELAY"
	Delayed              State = "DELAYED"
	ReturningToRealtime  State = "RETURNING_TO_REALTIME"
	Dumping              State = "DUMPING"
	SafeHold             State = "SAFE_HOLD"
	Error                State = "ERROR"
)

func (s State) Valid() bool {
	switch s {
	case Stopped, Starting, Ready, Ingesting, Realtime,
		BufferingToDelay, Delayed, ReturningToRealtime, Dumping, SafeHold, Error:
		return true
	default:
		return false
	}
}

func ParseState(raw string) (State, error) {
	s := State(raw)
	if !s.Valid() {
		return "", fmt.Errorf("unknown state: %s", raw)
	}
	return s, nil
}
