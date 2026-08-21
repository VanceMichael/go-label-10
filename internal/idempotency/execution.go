package idempotency

import (
	"sync"

	"github.com/VanceMichael/go-base-airbridge/internal/domain"
)

type executionPhase uint8

const (
	phaseExecuting executionPhase = iota + 1
	phaseCompleted
	phaseAborted
	phaseExpired
)

type execution struct {
	mu     sync.RWMutex
	record Record
	phase  executionPhase
	done   chan struct{}
	once   sync.Once
}

func newExecution(record Record) *execution {
	return &execution{
		record: record,
		phase:  phaseExecuting,
		done:   make(chan struct{}),
	}
}

func (state *execution) snapshot() Record {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := state.record
	result.Body = append([]byte(nil), state.record.Body...)
	return result
}

func (state *execution) complete(status int, body []byte) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.phase != phaseExecuting {
		return domain.ErrState
	}
	if status < 200 || status > 599 {
		return domain.ErrInvalid
	}
	state.record.Status = status
	state.record.Body = append([]byte(nil), body...)
	state.phase = phaseCompleted
	state.signal()
	return nil
}

func (state *execution) abort() {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.phase != phaseExecuting {
		return
	}
	state.phase = phaseAborted
	state.signal()
}

func (state *execution) expire() {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.phase == phaseExpired {
		return
	}
	state.phase = phaseExpired
	state.signal()
}

func (state *execution) signal() {
	state.once.Do(func() {
		close(state.done)
	})
}
