package memstepstore

import (
	"context"
	"sort"
	"sync"

	"github.com/lunarforge/workflow"
)

var _ workflow.StepStore = (*Store)(nil)

func New() *Store {
	return &Store{
		idIncrement: 1,
	}
}

type Store struct {
	mu          sync.Mutex
	idIncrement int64
	records     []*workflow.StepRecord
}

func (s *Store) Create(ctx context.Context, record *workflow.StepRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record.ID = s.idIncrement
	s.idIncrement++
	s.records = append(s.records, record)

	return nil
}

func (s *Store) Update(ctx context.Context, record *workflow.StepRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.records {
		if r.ID == record.ID {
			s.records[i] = record
			return nil
		}
	}

	return nil
}

func (s *Store) List(ctx context.Context, workflowName, runID string) ([]workflow.StepRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []workflow.StepRecord
	for _, r := range s.records {
		if r.WorkflowName == workflowName && r.RunID == runID {
			result = append(result, *r)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.Before(result[j].StartedAt)
	})

	return result, nil
}

func (s *Store) ListByStatus(ctx context.Context, workflowName, runID string, status int) ([]workflow.StepRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []workflow.StepRecord
	for _, r := range s.records {
		if r.WorkflowName == workflowName && r.RunID == runID && r.Status == status {
			result = append(result, *r)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.Before(result[j].StartedAt)
	})

	return result, nil
}
