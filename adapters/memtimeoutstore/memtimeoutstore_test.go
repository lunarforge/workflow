package memtimeoutstore_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
	"github.com/lunarforge/workflow/adapters/memtimeoutstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		return memtimeoutstore.New()
	})
}
