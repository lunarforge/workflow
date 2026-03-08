package natsstreamer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/luno/workflow"
)

// Translator converts a NATS JetStream message into a workflow ConnectorEvent.
type Translator func(msg jetstream.Msg) *workflow.ConnectorEvent

// NewConnector creates a ConnectorConstructor that consumes from a NATS JetStream subject
// and translates messages into workflow ConnectorEvents.
func NewConnector(nc *nats.Conn, subject string, t Translator, opts ...Option) *Connector {
	var o options
	o.maxDeliver = -1

	for _, opt := range opts {
		opt(&o)
	}

	return &Connector{
		nc:         nc,
		subject:    subject,
		translator: t,
		opts:       o,
	}
}

var _ workflow.ConnectorConstructor = (*Connector)(nil)

type Connector struct {
	nc         *nats.Conn
	subject    string
	translator Translator
	opts       options
}

func (c *Connector) Make(ctx context.Context, name string) (workflow.ConnectorConsumer, error) {
	js, err := jetstream.New(c.nc)
	if err != nil {
		return nil, err
	}

	streamName := streamNameFromTopic(c.subject)

	cfg := jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  []string{c.subject},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    72 * time.Hour,
		Storage:   jetstream.FileStorage,
	}

	if c.opts.streamConfig != nil {
		cfg = *c.opts.streamConfig
		cfg.Name = streamName
		cfg.Subjects = []string{c.subject}
	}

	_, err = js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ensure stream %q for connector subject %q: %w", streamName, c.subject, err)
	}

	consumerName := sanitizeName(name)
	consumerCfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: c.subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	if c.opts.maxDeliver > 0 {
		consumerCfg.MaxDeliver = c.opts.maxDeliver
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("create connector consumer %q: %w", consumerName, err)
	}

	consumeCtx, cancel := context.WithCancel(ctx)

	msgs, err := consumer.Messages(jetstream.PullMaxMessages(1))
	if err != nil {
		cancel()
		return nil, err
	}

	return &connectorConsumer{
		cancel:     cancel,
		ctx:        consumeCtx,
		msgs:       msgs,
		translator: c.translator,
	}, nil
}

type connectorConsumer struct {
	cancel     context.CancelFunc
	ctx        context.Context
	msgs       jetstream.MessagesContext
	translator Translator
}

var _ workflow.ConnectorConsumer = (*connectorConsumer)(nil)

func (c *connectorConsumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-c.ctx.Done():
			return nil, nil, c.ctx.Err()
		default:
		}

		msg, err := c.msgs.Next()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				return nil, nil, context.Canceled
			}
			return nil, nil, err
		}

		event := c.translator(msg)

		return event, func() error {
			return msg.Ack()
		}, nil
	}
}

func (c *connectorConsumer) Close() error {
	c.msgs.Stop()
	c.cancel()
	return nil
}
