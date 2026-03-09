package natsstreamer_test

import (
	"strconv"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	natscontainer "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/adaptertest"
	"github.com/lunarforge/workflow/adapters/natsstreamer"
)

func setupNATS(t *testing.T) *nats.Conn {
	t.Helper()
	ctx := t.Context()

	container, err := natscontainer.Run(ctx, "nats:2-alpine")
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	uri, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	nc, err := nats.Connect(uri)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	return nc
}

func TestStreamer(t *testing.T) {
	nc := setupNATS(t)

	adaptertest.RunEventStreamerTest(t, func() workflow.EventStreamer {
		streamer, err := natsstreamer.New(nc)
		require.NoError(t, err)
		return streamer
	})
}

func TestConnector(t *testing.T) {
	nc := setupNATS(t)
	subject := "connector-topic"

	translator := func(msg jetstream.Msg) *workflow.ConnectorEvent {
		meta, _ := msg.Metadata()
		return &workflow.ConnectorEvent{
			ID:        strconv.FormatUint(meta.Sequence.Stream, 10),
			ForeignID: msg.Headers().Get("foreign_id"),
			Type:      subject,
			CreatedAt: meta.Timestamp,
		}
	}

	connector := natsstreamer.NewConnector(nc, subject, translator)

	adaptertest.RunConnectorTest(t, func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor {
		js, err := jetstream.New(nc)
		require.NoError(t, err)

		ctx := t.Context()

		// Ensure stream exists before publishing.
		_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
			Name:     "connector-topic",
			Subjects: []string{subject},
		})
		require.NoError(t, err)

		for _, e := range seedEvents {
			msg := &nats.Msg{
				Subject: subject,
				Data:    []byte(e.Type),
				Header:  nats.Header{},
			}
			msg.Header.Set("foreign_id", e.ForeignID)

			_, err := js.PublishMsg(ctx, msg)
			require.NoError(t, err)
		}

		return connector
	})
}
