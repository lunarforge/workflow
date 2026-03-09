package pgstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/lunarforge/workflow"
)

type Store struct {
	db *bun.DB
}

func New(db *bun.DB) *Store {
	return &Store{db: db}
}

var _ workflow.RecordStore = (*Store)(nil)

func (s *Store) Store(ctx context.Context, r *workflow.Record) error {
	metaBytes, err := json.Marshal(r.Meta)
	if err != nil {
		return err
	}

	now := time.Now()

	record := &RecordModel{
		WorkflowName: r.WorkflowName,
		ForeignID:    r.ForeignID,
		RunID:        r.RunID,
		RunState:     int(r.RunState),
		Status:       r.Status,
		Object:       r.Object,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    now,
		Meta:         metaBytes,
	}

	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}

	eventData, err := workflow.MakeOutboxEventData(*r)
	if err != nil {
		return err
	}

	outbox := &OutboxModel{
		ID:           eventData.ID,
		WorkflowName: eventData.WorkflowName,
		Data:         eventData.Data,
		CreatedAt:    now,
	}

	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Upsert record: insert or update on run_id conflict.
		_, err := tx.NewInsert().
			Model(record).
			On("CONFLICT (run_id) DO UPDATE").
			Set("run_state = EXCLUDED.run_state").
			Set("status = EXCLUDED.status").
			Set("object = EXCLUDED.object").
			Set("updated_at = EXCLUDED.updated_at").
			Set("meta = EXCLUDED.meta").
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewInsert().
			Model(outbox).
			Exec(ctx)
		return err
	})
}

func (s *Store) Lookup(ctx context.Context, runID string) (*workflow.Record, error) {
	var m RecordModel
	err := s.db.NewSelect().
		Model(&m).
		Where("run_id = ?", runID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, workflow.ErrRecordNotFound
		}
		return nil, err
	}

	return modelToRecord(&m)
}

func (s *Store) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	var m RecordModel
	err := s.db.NewSelect().
		Model(&m).
		Where("workflow_name = ?", workflowName).
		Where("foreign_id = ?", foreignID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, workflow.ErrRecordNotFound
		}
		return nil, err
	}

	return modelToRecord(&m)
}

func (s *Store) List(
	ctx context.Context,
	workflowName string,
	offset int64,
	limit int,
	order workflow.OrderType,
	filters ...workflow.RecordFilter,
) ([]workflow.Record, error) {
	if limit == 0 {
		limit = 25
	}

	filter := workflow.MakeFilter(filters...)

	q := s.db.NewSelect().
		Model((*RecordModel)(nil)).
		Where("run_id IS NOT NULL")

	if workflowName != "" {
		q = q.Where("workflow_name = ?", workflowName)
	}

	if f := filter.ByForeignID(); f.Enabled {
		if f.IsMultiMatch {
			q = q.Where("foreign_id IN (?)", bun.In(toAnySlice(f.MultiValues())))
		} else {
			q = q.Where("foreign_id = ?", f.Value())
		}
	}

	if f := filter.ByStatus(); f.Enabled {
		if f.IsMultiMatch {
			q = q.Where("status IN (?)", bun.In(toAnySlice(f.MultiValues())))
		} else {
			q = q.Where("status = ?", f.Value())
		}
	}

	if f := filter.ByRunState(); f.Enabled {
		if f.IsMultiMatch {
			q = q.Where("run_state IN (?)", bun.In(toAnySlice(f.MultiValues())))
		} else {
			q = q.Where("run_state = ?", f.Value())
		}
	}

	if f := filter.ByCreatedAtAfter(); f.Enabled {
		q = q.Where("created_at > ?", f.Value())
	}

	if f := filter.ByCreatedAtBefore(); f.Enabled {
		q = q.Where("created_at < ?", f.Value())
	}

	q = q.OrderExpr("created_at " + order.String())

	if offset > 0 {
		q = q.Offset(int(offset))
	}
	q = q.Limit(limit)

	var models []RecordModel
	err := q.Scan(ctx, &models)
	if err != nil {
		return nil, err
	}

	records := make([]workflow.Record, 0, len(models))
	for i := range models {
		r, err := modelToRecord(&models[i])
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}

	return records, nil
}

func (s *Store) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]workflow.OutboxEvent, error) {
	var models []OutboxModel
	err := s.db.NewSelect().
		Model(&models).
		Where("workflow_name = ?", workflowName).
		Limit(int(limit)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]workflow.OutboxEvent, 0, len(models))
	for _, m := range models {
		events = append(events, workflow.OutboxEvent{
			ID:           m.ID,
			WorkflowName: m.WorkflowName,
			Data:         m.Data,
			CreatedAt:    m.CreatedAt,
		})
	}

	return events, nil
}

func (s *Store) DeleteOutboxEvent(ctx context.Context, id string) error {
	_, err := s.db.NewDelete().
		Model((*OutboxModel)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func modelToRecord(m *RecordModel) (*workflow.Record, error) {
	r := &workflow.Record{
		WorkflowName: m.WorkflowName,
		ForeignID:    m.ForeignID,
		RunID:        m.RunID,
		RunState:     workflow.RunState(m.RunState),
		Status:       m.Status,
		Object:       m.Object,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}

	if len(m.Meta) > 0 {
		err := json.Unmarshal(m.Meta, &r.Meta)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

func toAnySlice(values []string) []any {
	result := make([]any, len(values))
	for i, v := range values {
		result[i] = v
	}
	return result
}
