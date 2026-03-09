package memstepstore_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
	"github.com/lunarforge/workflow/adapters/memstepstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunStepStoreTest(t, func() workflow.StepStore {
		return memstepstore.New()
	})
}
