package sqlstore_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"

	"github.com/lunarforge/workflow/adapters/sqlstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		dbc := ConnectForTesting(t)
		return sqlstore.New(dbc, dbc, "workflow_records", "workflow_outbox")
	})
}
