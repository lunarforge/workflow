package pgstore

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/lunarforge/workflow"
)

type TimeoutStore struct {
	db *bun.DB
}

func NewTimeoutStore(db *bun.DB) *TimeoutStore {
	return &TimeoutStore{db: db}
}

var _ workflow.TimeoutStore = (*TimeoutStore)(nil)

func (s *TimeoutStore) Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error {
	m := &TimeoutModel{
		WorkflowName: workflowName,
		ForeignID:    foreignID,
		RunID:        runID,
		Status:       status,
		Completed:    false,
		ExpireAt:     expireAt,
		CreatedAt:    time.Now(),
	}

	_, err := s.db.NewInsert().Model(m).Exec(ctx)
	return err
}

func (s *TimeoutStore) Complete(ctx context.Context, id int64) error {
	_, err := s.db.NewUpdate().
		Model((*TimeoutModel)(nil)).
		Set("completed = ?", true).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *TimeoutStore) Cancel(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*TimeoutModel)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *TimeoutStore) List(ctx context.Context, workflowName string) ([]workflow.TimeoutRecord, error) {
	var models []TimeoutModel
	err := s.db.NewSelect().
		Model(&models).
		Where("workflow_name = ?", workflowName).
		Where("completed = ?", false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return modelsToTimeouts(models), nil
}

func (s *TimeoutStore) ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]workflow.TimeoutRecord, error) {
	var models []TimeoutModel
	err := s.db.NewSelect().
		Model(&models).
		Where("workflow_name = ?", workflowName).
		Where("status = ?", status).
		Where("expire_at < ?", now).
		Where("completed = ?", false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return modelsToTimeouts(models), nil
}

func modelsToTimeouts(models []TimeoutModel) []workflow.TimeoutRecord {
	records := make([]workflow.TimeoutRecord, 0, len(models))
	for _, m := range models {
		records = append(records, workflow.TimeoutRecord{
			ID:           m.ID,
			WorkflowName: m.WorkflowName,
			ForeignID:    m.ForeignID,
			RunID:        m.RunID,
			Status:       m.Status,
			Completed:    m.Completed,
			ExpireAt:     m.ExpireAt,
			CreatedAt:    m.CreatedAt,
		})
	}
	return records
}
