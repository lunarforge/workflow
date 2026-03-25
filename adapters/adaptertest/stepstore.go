package adaptertest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lunarforge/workflow"
)

func RunStepStoreTest(t *testing.T, factory func() workflow.StepStore) {
	tests := []func(t *testing.T, factory func() workflow.StepStore){
		testStepStoreCreateAndList,
		testStepStoreUpdate,
		testStepStoreListByStatus,
		testStepStoreListOrdering,
	}

	for _, test := range tests {
		test(t, factory)
	}
}

func testStepStoreCreateAndList(t *testing.T, factory func() workflow.StepStore) {
	t.Run("Create and List step records", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		now := time.Now()
		rec := &workflow.StepRecord{
			WorkflowName:      "example",
			ForeignID:         "user-123",
			RunID:             "run-1",
			Status:            int(statusStarted),
			StatusDescription: "Started",
			StepStatus:        workflow.StepStatusStarted,
			Attempt:           1,
			StartedAt:         now,
		}

		err := store.Create(ctx, rec)
		require.NoError(t, err)
		require.NotZero(t, rec.ID)

		records, err := store.List(ctx, "example", "run-1")
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, "example", records[0].WorkflowName)
		require.Equal(t, "run-1", records[0].RunID)
		require.Equal(t, int(statusStarted), records[0].Status)
		require.Equal(t, workflow.StepStatusStarted, records[0].StepStatus)
		require.Equal(t, 1, records[0].Attempt)

		// List for different run returns empty
		records, err = store.List(ctx, "example", "run-other")
		require.NoError(t, err)
		require.Len(t, records, 0)
	})
}

func testStepStoreUpdate(t *testing.T, factory func() workflow.StepStore) {
	t.Run("Update step record", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		now := time.Now()
		rec := &workflow.StepRecord{
			WorkflowName:      "example",
			ForeignID:         "user-123",
			RunID:             "run-1",
			Status:            int(statusStarted),
			StatusDescription: "Started",
			StepStatus:        workflow.StepStatusStarted,
			Attempt:           1,
			StartedAt:         now,
		}

		err := store.Create(ctx, rec)
		require.NoError(t, err)

		finishedAt := now.Add(500 * time.Millisecond)
		rec.StepStatus = workflow.StepStatusSucceeded
		rec.FinishedAt = &finishedAt
		rec.Duration = 500 * time.Millisecond
		rec.ToStatus = int(statusMiddle)

		err = store.Update(ctx, rec)
		require.NoError(t, err)

		records, err := store.List(ctx, "example", "run-1")
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, workflow.StepStatusSucceeded, records[0].StepStatus)
		require.NotNil(t, records[0].FinishedAt)
		require.Equal(t, 500*time.Millisecond, records[0].Duration)
		require.Equal(t, int(statusMiddle), records[0].ToStatus)
	})
}

func testStepStoreListByStatus(t *testing.T, factory func() workflow.StepStore) {
	t.Run("ListByStatus filters correctly", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		now := time.Now()

		// Create two step records with different statuses
		err := store.Create(ctx, &workflow.StepRecord{
			WorkflowName: "example",
			ForeignID:    "user-123",
			RunID:        "run-1",
			Status:       int(statusStarted),
			StepStatus:   workflow.StepStatusSucceeded,
			Attempt:      1,
			StartedAt:    now,
		})
		require.NoError(t, err)

		err = store.Create(ctx, &workflow.StepRecord{
			WorkflowName: "example",
			ForeignID:    "user-123",
			RunID:        "run-1",
			Status:       int(statusMiddle),
			StepStatus:   workflow.StepStatusStarted,
			Attempt:      1,
			StartedAt:    now.Add(time.Second),
		})
		require.NoError(t, err)

		records, err := store.ListByStatus(ctx, "example", "run-1", int(statusStarted))
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, int(statusStarted), records[0].Status)

		records, err = store.ListByStatus(ctx, "example", "run-1", int(statusMiddle))
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, int(statusMiddle), records[0].Status)
	})
}

func testStepStoreListOrdering(t *testing.T, factory func() workflow.StepStore) {
	t.Run("List returns records ordered by StartedAt", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		now := time.Now()

		// Insert in reverse order
		err := store.Create(ctx, &workflow.StepRecord{
			WorkflowName: "example",
			RunID:        "run-1",
			Status:       int(statusMiddle),
			Attempt:      1,
			StartedAt:    now.Add(2 * time.Second),
		})
		require.NoError(t, err)

		err = store.Create(ctx, &workflow.StepRecord{
			WorkflowName: "example",
			RunID:        "run-1",
			Status:       int(statusStarted),
			Attempt:      1,
			StartedAt:    now,
		})
		require.NoError(t, err)

		records, err := store.List(ctx, "example", "run-1")
		require.NoError(t, err)
		require.Len(t, records, 2)
		require.True(t, records[0].StartedAt.Before(records[1].StartedAt))
	})
}
