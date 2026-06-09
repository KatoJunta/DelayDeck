package state

// ForwardingCoordinator drives control transitions against a live media pipeline.
// When nil, the state machine uses timed mock transitions for mock mode.
type ForwardingCoordinator interface {
	BeginEnableDelay(targetSeconds int) error
	RunEnableDelayFill(targetSeconds int, publish func(slate string, countdown int))
	CompleteEnableDelay()

	BeginReturnLive() error
	RunReturnLiveDrain(publish func(slate string, countdown int))
	RunReturnLiveSlate(publish func(slate string, countdown int))
	ResumePassthrough()

	BeginDumpSlate() error
	RunDumpSlate(publish func(slate string, countdown int))
}

// EnableDelayCoordinator is kept for callers that only wire delay-on.
type EnableDelayCoordinator = ForwardingCoordinator
