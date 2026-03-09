package gettingstarted_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memrolescheduler"
	"github.com/lunarforge/workflow/adapters/memstreamer"
	"github.com/lunarforge/workflow/adapters/memtimeoutstore"
	"github.com/stretchr/testify/require"

	"github.com/lunarforge/workflow/_examples/gettingstarted"
)

func TestWorkflow(t *testing.T) {
	wf := gettingstarted.Workflow(gettingstarted.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
	t.Cleanup(wf.Stop)

	ctx := t.Context()
	wf.Run(ctx)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID)
	require.NoError(t, err)

	workflow.Require(t, wf, foreignID, gettingstarted.StatusReadTheDocs, gettingstarted.GettingStarted{
		ReadTheDocs: "✅",
	})

	workflow.Require(t, wf, foreignID, gettingstarted.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})

	workflow.Require(t, wf, foreignID, gettingstarted.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})
}
