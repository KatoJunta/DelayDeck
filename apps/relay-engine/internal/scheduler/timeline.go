package scheduler

import "sync"

type trackTimeline struct {
	lastOut    uint32
	lastSource uint32
}

func (t *trackTimeline) mapSource(source uint32) uint32 {
	if t.lastSource != 0 && source >= t.lastSource {
		delta := source - t.lastSource
		if delta == 0 {
			delta = 1
		}
		out := t.lastOut + delta
		t.lastOut = out
		t.lastSource = source
		return out
	}

	if t.lastOut == 0 {
		t.lastOut = source
		t.lastSource = source
		return source
	}

	out := t.lastOut + 1
	t.lastOut = out
	t.lastSource = source
	return out
}

func (t *trackTimeline) advance(delta uint32) uint32 {
	if delta == 0 {
		delta = 1
	}
	if t.lastOut == 0 {
		t.lastOut = delta
	} else {
		t.lastOut += delta
	}
	t.lastSource = t.lastOut
	return t.lastOut
}

type OutputTimeline struct {
	mu    sync.Mutex
	video trackTimeline
	audio trackTimeline
}

func (t *OutputTimeline) Video(source uint32) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.video.mapSource(source)
}

func (t *OutputTimeline) Audio(source uint32) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.audio.mapSource(source)
}

func (t *OutputTimeline) AdvanceVideo(deltaMs uint32) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.video.advance(deltaMs)
}

func (t *OutputTimeline) AdvanceAudio(deltaMs uint32) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.audio.advance(deltaMs)
}

func (t *OutputTimeline) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.video = trackTimeline{}
	t.audio = trackTimeline{}
}
