package agentic_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewActivityManager ---

func TestNewActivityManager(t *testing.T) {
	am := agentic.NewActivityManager()
	require.NotNil(t, am)
	assert.False(t, am.IsUserActive())
	assert.False(t, am.IsCLIActive())
}

// --- RecordUserActivity ---

func TestActivityManager_RecordUserActivity(t *testing.T) {
	am := agentic.NewActivityManager()
	am.RecordUserActivity()
	assert.True(t, am.IsUserActive())
}

func TestActivityManager_UserActivityExpires(t *testing.T) {
	am := agentic.NewActivityManagerWithTimeout(50 * time.Millisecond)
	am.RecordUserActivity()
	assert.True(t, am.IsUserActive())

	time.Sleep(100 * time.Millisecond)
	assert.False(t, am.IsUserActive())
}

// --- CLI Activity ---

func TestActivityManager_StartEndCLIActivity(t *testing.T) {
	am := agentic.NewActivityManager()

	stop := am.StartCLIActivity("op-1")
	assert.True(t, am.IsCLIActive())
	assert.Equal(t, 1, am.ActiveOperationCount())

	stop()
	assert.False(t, am.IsCLIActive())
	assert.Equal(t, 0, am.ActiveOperationCount())
}

func TestActivityManager_MultipleCLIActivities(t *testing.T) {
	am := agentic.NewActivityManager()

	stop1 := am.StartCLIActivity("op-1")
	stop2 := am.StartCLIActivity("op-2")
	assert.Equal(t, 2, am.ActiveOperationCount())

	stop1()
	assert.True(t, am.IsCLIActive())
	assert.Equal(t, 1, am.ActiveOperationCount())

	stop2()
	assert.False(t, am.IsCLIActive())
}

func TestActivityManager_EndCLIActivity_Direct(t *testing.T) {
	am := agentic.NewActivityManager()
	am.StartCLIActivity("op-1")
	am.EndCLIActivity("op-1")
	assert.False(t, am.IsCLIActive())
}

func TestActivityManager_EndCLIActivity_Nonexistent(t *testing.T) {
	am := agentic.NewActivityManager()
	am.EndCLIActivity("nonexistent") // should not panic
	assert.False(t, am.IsCLIActive())
}

// --- TrackOperation ---

func TestActivityManager_TrackOperation_Success(t *testing.T) {
	am := agentic.NewActivityManager()
	var duringActive bool

	err := am.TrackOperation("op-1", func() error {
		duringActive = am.IsCLIActive()
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, duringActive)
	assert.False(t, am.IsCLIActive(), "should be inactive after operation")
}

func TestActivityManager_TrackOperation_Error(t *testing.T) {
	am := agentic.NewActivityManager()
	expected := errors.New("fail")

	err := am.TrackOperation("op-1", func() error {
		return expected
	})

	assert.Equal(t, expected, err)
	assert.False(t, am.IsCLIActive(), "should cleanup even on error")
}

// --- GetState ---

func TestActivityManager_GetState(t *testing.T) {
	am := agentic.NewActivityManager()
	am.RecordUserActivity()
	am.StartCLIActivity("op-1")

	state := am.GetState()
	assert.True(t, state.UserActive)
	assert.True(t, state.CLIActive)
	assert.Len(t, state.ActiveOperations, 1)
	assert.Contains(t, state.ActiveOperations, "op-1")
	assert.False(t, state.LastUserActivity.IsZero())
}

func TestActivityManager_GetState_Empty(t *testing.T) {
	am := agentic.NewActivityManager()
	state := am.GetState()
	assert.False(t, state.UserActive)
	assert.False(t, state.CLIActive)
	assert.Empty(t, state.ActiveOperations)
}

// --- Subscribe ---

func TestActivityManager_Subscribe(t *testing.T) {
	am := agentic.NewActivityManager()

	var states []agentic.ActivityState
	am.Subscribe(func(s agentic.ActivityState) {
		states = append(states, s)
	})

	am.RecordUserActivity()
	am.StartCLIActivity("op-1")

	assert.Len(t, states, 2)
	assert.True(t, states[0].UserActive)
	assert.True(t, states[1].CLIActive)
}

func TestActivityManager_SubscribeUnsubscribe(t *testing.T) {
	am := agentic.NewActivityManager()

	var count int
	unsub := am.Subscribe(func(s agentic.ActivityState) {
		count++
	})

	am.RecordUserActivity()
	assert.Equal(t, 1, count)

	unsub()
	am.RecordUserActivity()
	assert.Equal(t, 1, count, "should not fire after unsubscribe")
}

// --- Reset ---

func TestActivityManager_Reset(t *testing.T) {
	am := agentic.NewActivityManager()
	am.RecordUserActivity()
	am.StartCLIActivity("op-1")

	am.Reset()

	assert.False(t, am.IsUserActive())
	assert.False(t, am.IsCLIActive())
	assert.Equal(t, 0, am.ActiveOperationCount())
}

// --- Concurrency ---

func TestActivityManager_ConcurrentAccess(t *testing.T) {
	am := agentic.NewActivityManager()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			am.RecordUserActivity()
		}()
		go func(id int) {
			defer wg.Done()
			stop := am.StartCLIActivity(string(rune('A' + id)))
			time.Sleep(time.Millisecond)
			stop()
		}(i)
	}
	wg.Wait()

	assert.False(t, am.IsCLIActive())
}
