package callback_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memrolescheduler"
	"github.com/lunarforge/workflow/adapters/memstreamer"
	"github.com/stretchr/testify/require"

	"github.com/lunarforge/workflow/_examples/callback"
)

func TestCallbackWorkflow(t *testing.T) {
	wf := callback.ExampleWorkflow(callback.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
	t.Cleanup(wf.Stop)

	ctx := t.Context()
	wf.Run(ctx)

	foreignID := "andrew"
	runID, err := wf.Trigger(ctx, foreignID)
	require.NoError(t, err)

	workflow.TriggerCallbackOn(t, wf, foreignID, runID, callback.StatusStarted, callback.EmailConfirmationResponse{
		Confirmed: true,
	})

	workflow.Require(t, wf, foreignID, callback.StatusFollowedTheExample, callback.Example{
		EmailConfirmed: true,
	})
}
