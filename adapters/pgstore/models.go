package pgstore

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type RecordModel struct {
	bun.BaseModel `bun:"table:workflow_records,alias:r"`

	ID           int64     `bun:",pk,autoincrement"`
	WorkflowName string    `bun:",notnull"`
	ForeignID    string    `bun:",notnull"`
	RunID        string    `bun:",notnull,unique"`
	RunState     int       `bun:",notnull"`
	Status       int       `bun:",notnull"`
	Object       []byte    `bun:"type:bytea"`
	CreatedAt    time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	Meta         []byte    `bun:"type:bytea"`
}

type OutboxModel struct {
	bun.BaseModel `bun:"table:workflow_outbox,alias:o"`

	ID           string    `bun:",pk"`
	WorkflowName string    `bun:",notnull"`
	Data         []byte    `bun:"type:bytea"`
	CreatedAt    time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

type TimeoutModel struct {
	bun.BaseModel `bun:"table:workflow_timeouts,alias:t"`

	ID           int64     `bun:",pk,autoincrement"`
	WorkflowName string    `bun:",notnull"`
	ForeignID    string    `bun:",notnull"`
	RunID        string    `bun:",notnull"`
	Status       int       `bun:",notnull"`
	Completed    bool      `bun:",notnull,default:false"`
	ExpireAt     time.Time `bun:",notnull"`
	CreatedAt    time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

// CreateTables creates all workflow tables if they do not exist.
func CreateTables(ctx context.Context, db *bun.DB) error {
	models := []interface{}{
		(*RecordModel)(nil),
		(*OutboxModel)(nil),
		(*TimeoutModel)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	// Create indexes for common query patterns.
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_workflow_records_workflow_foreign ON workflow_records (workflow_name, foreign_id)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_records_workflow_status ON workflow_records (workflow_name, status)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_records_created_at ON workflow_records (created_at)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_outbox_workflow ON workflow_outbox (workflow_name)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_timeouts_workflow_status ON workflow_timeouts (workflow_name, status, completed)",
	}

	for _, idx := range indexes {
		_, err := db.ExecContext(ctx, idx)
		if err != nil {
			return err
		}
	}

	return nil
}
