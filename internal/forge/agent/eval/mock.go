package eval

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
)

// ToolCaller abstracts tool invocation so we can intercept calls.
type ToolCaller interface {
	CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error)
}

// ServerManagerCaller adapts servermgr.Manager to the ToolCaller interface.
type ServerManagerCaller struct {
	Mgr *servermgr.Manager
}

func (s *ServerManagerCaller) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	return s.Mgr.CallTool(ctx, name, args)
}

// MockInjector wraps a ToolCaller to inject failures for configured tools.
type MockInjector struct {
	inner  ToolCaller
	mocks  map[string]*mockState
	mu     sync.Mutex
}

type mockState struct {
	errorMsg string
	maxCount int // 0 = always fail
	count    int // how many times we've failed so far
}

// NewMockInjector creates a MockInjector that wraps the given ToolCaller.
// If mockErrors is empty, calls pass through unmodified.
func NewMockInjector(inner ToolCaller, mockErrors []MockError) *MockInjector {
	mocks := make(map[string]*mockState, len(mockErrors))
	for _, me := range mockErrors {
		mocks[me.Tool] = &mockState{
			errorMsg: me.Error,
			maxCount: me.Count,
		}
	}
	return &MockInjector{
		inner: inner,
		mocks: mocks,
	}
}

// CallTool intercepts tool calls, returning mock errors when configured.
func (m *MockInjector) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	m.mu.Lock()
	state, hasMock := m.mocks[name]
	if hasMock {
		shouldFail := state.maxCount == 0 || state.count < state.maxCount
		if shouldFail {
			state.count++
			m.mu.Unlock()
			return nil, 0, fmt.Errorf("%s", state.errorMsg)
		}
	}
	m.mu.Unlock()

	return m.inner.CallTool(ctx, name, args)
}

// Reset clears failure counters so the injector can be reused across runs.
func (m *MockInjector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.mocks {
		s.count = 0
	}
}
