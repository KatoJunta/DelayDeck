package ingest

import (
	"sync"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type ForwardingCoordinator struct {
	mu               sync.Mutex
	pipeline         *Pipeline
	sessionCounter   uint64
	operationSession uint64
}

func NewForwardingCoordinator() *ForwardingCoordinator {
	return &ForwardingCoordinator{}
}

func (c *ForwardingCoordinator) SetPipeline(pipeline *Pipeline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionCounter++
	if pipeline != nil {
		pipeline.sessionID = c.sessionCounter
	}
	c.pipeline = pipeline
	c.operationSession = 0
}

func (c *ForwardingCoordinator) ClearPipeline() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pipeline = nil
	c.operationSession = 0
}

func (c *ForwardingCoordinator) bindOperationSession() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline == nil {
		return errNoActiveSession
	}
	c.operationSession = c.pipeline.sessionID
	return nil
}

func (c *ForwardingCoordinator) activePipelineForOperation() *Pipeline {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline == nil || c.pipeline.sessionID != c.operationSession {
		return nil
	}
	return c.pipeline
}

func (c *ForwardingCoordinator) activePipeline() *Pipeline {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pipeline
}

var errNoActiveSession = stateTransitionError("no active ingest session")

type stateTransitionError string

func (e stateTransitionError) Error() string {
	return string(e)
}

func (c *ForwardingCoordinator) BeginEnableDelay(targetSeconds int) error {
	pipeline := c.activePipeline()
	if pipeline == nil {
		return errNoActiveSession
	}
	if err := pipeline.BeginEnableDelay(targetSeconds); err != nil {
		return err
	}
	return c.bindOperationSession()
}

func (c *ForwardingCoordinator) RunEnableDelayFill(targetSeconds int, publish func(slate string, countdown int)) {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.RunEnableDelayFill(targetSeconds, publish)
}

func (c *ForwardingCoordinator) CompleteEnableDelay() {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.CompleteEnableDelay()
}

func (c *ForwardingCoordinator) BeginReturnLive() error {
	pipeline := c.activePipeline()
	if pipeline == nil {
		return errNoActiveSession
	}
	if err := pipeline.BeginReturnLive(); err != nil {
		return err
	}
	return c.bindOperationSession()
}

func (c *ForwardingCoordinator) RunReturnLiveDrain(publish func(slate string, countdown int)) {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.RunReturnLiveDrain(publish)
}

func (c *ForwardingCoordinator) RunReturnLiveSlate(publish func(slate string, countdown int)) {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.RunReturnLiveSlate(publish)
}

func (c *ForwardingCoordinator) ResumePassthrough() {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.ResumePassthrough()
}

func (c *ForwardingCoordinator) BeginDumpSlate() error {
	pipeline := c.activePipeline()
	if pipeline == nil {
		return errNoActiveSession
	}
	if err := pipeline.BeginDumpSlate(); err != nil {
		return err
	}
	return c.bindOperationSession()
}

func (c *ForwardingCoordinator) RunDumpSlate(publish func(slate string, countdown int)) {
	pipeline := c.activePipelineForOperation()
	if pipeline == nil {
		return
	}
	pipeline.RunDumpSlate(publish)
}

var _ state.ForwardingCoordinator = (*ForwardingCoordinator)(nil)
