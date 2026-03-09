package pgstore_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
	"github.com/lunarforge/workflow/adapters/pgstore"
)

func setupDB(t *testing.T) *bun.DB {
	t.Helper()
	ctx := t.Context()

	container, err := pgcontainer.Run(ctx, "postgres:16-alpine",
		pgcontainer.WithDatabase("workflow_test"),
		pgcontainer.WithUsername("test"),
		pgcontainer.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp"),
		),
	)
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(connStr)))
	db := bun.NewDB(sqldb, pgdialect.New())

	err = pgstore.CreateTables(ctx, db)
	require.NoError(t, err)

	return db
}

func TestRecordStore(t *testing.T) {
	db := setupDB(t)

	factory := func() workflow.RecordStore {
		// Clean tables between test runs.
		ctx := t.Context()
		_, _ = db.NewTruncateTable().Model((*pgstore.OutboxModel)(nil)).Cascade().Exec(ctx)
		_, _ = db.NewTruncateTable().Model((*pgstore.RecordModel)(nil)).Cascade().Exec(ctx)

		return pgstore.New(db)
	}

	adaptertest.RunRecordStoreTest(t, factory)
}

func TestTimeoutStore(t *testing.T) {
	db := setupDB(t)

	factory := func() workflow.TimeoutStore {
		// Clean tables between test runs.
		ctx := t.Context()
		_, _ = db.NewTruncateTable().Model((*pgstore.TimeoutModel)(nil)).Cascade().Exec(ctx)

		return pgstore.NewTimeoutStore(db)
	}

	adaptertest.RunTimeoutStoreTest(t, factory)
}
