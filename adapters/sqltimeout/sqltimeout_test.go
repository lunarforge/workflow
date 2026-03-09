package sqltimeout_test

import (
	"github.com/lunarforge/workflow/adapters/sqltimeout"
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
)

func TestStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		dbc := ConnectForTesting(t)
		return sqltimeout.New(dbc, dbc, "workflow_timeouts")
	})
}
