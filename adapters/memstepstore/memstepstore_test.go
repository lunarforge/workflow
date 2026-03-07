package memstepstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memstepstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunStepStoreTest(t, func() workflow.StepStore {
		return memstepstore.New()
	})
}
