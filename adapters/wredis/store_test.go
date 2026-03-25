package wredis_test

import (
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/lunarforge/workflow/adapters/wredis"
)

func TestRedisRecordStore(t *testing.T) {
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})

	factory := func() workflow.RecordStore {
		// Clean the database before each test
		err := client.FlushDB(ctx).Err()
		require.NoError(t, err)

		return wredis.New(client)
	}

	adaptertest.RunRecordStoreTest(t, factory)
}
