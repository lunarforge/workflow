package natsstreamer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/luno/workflow"
)

type Option func(*options)

type options struct {
	streamConfig *jetstream.StreamConfig
	maxDeliver   int
}

// WithStreamConfig overrides the default JetStream stream config used when auto-creating streams.
// The Subjects field will be set automatically based on the topic.
func WithStreamConfig(cfg jetstream.StreamConfig) Option {
	return func(o *options) {
		o.streamConfig = &cfg
	}
}

// WithMaxDeliver sets the maximum number of delivery attempts before a message is dropped.
// Defaults to -1 (unlimited).
func WithMaxDeliver(n int) Option {
	return func(o *options) {
		o.maxDeliver = n
	}
}

func New(nc *nats.Conn, opts ...Option) (*StreamConstructor, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	var o options
	o.maxDeliver = -1

	for _, opt := range opts {
		opt(&o)
	}

	return &StreamConstructor{
		js:   js,
		opts: o,
	}, nil
}

var _ workflow.EventStreamer = (*StreamConstructor)(nil)

type StreamConstructor struct {
	js   jetstream.JetStream
	opts options
}

func (s *StreamConstructor) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	err := s.ensureStream(ctx, topic)
	if err != nil {
		return nil, err
	}

	return &Sender{
		js:    s.js,
		topic: topic,
	}, nil
}

func (s *StreamConstructor) NewReceiver(
	ctx context.Context,
	topic string,
	name string,
	opts ...workflow.ReceiverOption,
) (workflow.EventReceiver, error) {
	var ropts workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&ropts)
	}

	err := s.ensureStream(ctx, topic)
	if err != nil {
		return nil, err
	}

	consumerName := sanitizeName(name)
	streamName := streamNameFromTopic(topic)

	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: topic,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	if s.opts.maxDeliver > 0 {
		cfg.MaxDeliver = s.opts.maxDeliver
	}

	// Check if this consumer already exists to handle StreamFromLatest correctly.
	// StreamFromLatest should only take effect for new consumers with no committed offset.
	if ropts.StreamFromLatest {
		_, err := s.js.Consumer(ctx, streamName, consumerName)
		if errors.Is(err, jetstream.ErrConsumerNotFound) {
			cfg.DeliverPolicy = jetstream.DeliverNewPolicy
		}
		// If consumer exists, keep DeliverAll — the existing offset will be used.
	}

	consumer, err := s.js.CreateOrUpdateConsumer(ctx, streamName, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer %q on stream %q: %w", consumerName, streamName, err)
	}

	consumeCtx, cancel := context.WithCancel(ctx)

	msgs, err := consumer.Messages(jetstream.PullMaxMessages(1))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start message consumption: %w", err)
	}

	return &Receiver{
		cancel: cancel,
		ctx:    consumeCtx,
		msgs:   msgs,
	}, nil
}

func (s *StreamConstructor) ensureStream(ctx context.Context, topic string) error {
	streamName := streamNameFromTopic(topic)

	cfg := jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  []string{topic},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    72 * time.Hour,
		Storage:   jetstream.FileStorage,
	}

	if s.opts.streamConfig != nil {
		cfg = *s.opts.streamConfig
		cfg.Name = streamName
		cfg.Subjects = []string{topic}
	}

	_, err := s.js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return fmt.Errorf("ensure stream %q for topic %q: %w", streamName, topic, err)
	}

	return nil
}

type Sender struct {
	js    jetstream.JetStream
	topic string
}

var _ workflow.EventSender = (*Sender)(nil)

func (s *Sender) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	msg := &nats.Msg{
		Subject: s.topic,
		Data:    []byte(strconv.FormatInt(int64(statusType), 10)),
		Header:  nats.Header{},
	}

	for key, value := range headers {
		msg.Header.Set(string(key), value)
	}
	msg.Header.Set("foreign_id", foreignID)

	_, err := s.js.PublishMsg(ctx, msg)
	return err
}

func (s *Sender) Close() error {
	return nil
}

type Receiver struct {
	cancel context.CancelFunc
	ctx    context.Context
	msgs   jetstream.MessagesContext
}

var _ workflow.EventReceiver = (*Receiver)(nil)

func (r *Receiver) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-r.ctx.Done():
			return nil, nil, r.ctx.Err()
		default:
		}

		msg, err := r.msgs.Next()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				return nil, nil, context.Canceled
			}
			return nil, nil, err
		}

		event, err := msgToEvent(msg)
		if err != nil {
			// Terminate the message so it doesn't block the consumer.
			_ = msg.Term()
			return nil, nil, err
		}

		return event, func() error {
			return msg.Ack()
		}, nil
	}
}

func (r *Receiver) Close() error {
	r.msgs.Stop()
	r.cancel()
	return nil
}

func msgToEvent(msg jetstream.Msg) (*workflow.Event, error) {
	statusType, err := strconv.ParseInt(string(msg.Data()), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse status type from message data: %w", err)
	}

	headers := make(map[workflow.Header]string)
	for key, values := range msg.Headers() {
		if len(values) > 0 {
			headers[workflow.Header(key)] = values[0]
		}
	}

	foreignID := msg.Headers().Get("foreign_id")

	meta, err := msg.Metadata()
	if err != nil {
		return nil, fmt.Errorf("get message metadata: %w", err)
	}

	return &workflow.Event{
		ID:        int64(meta.Sequence.Stream),
		ForeignID: foreignID,
		Type:      int(statusType),
		Headers:   headers,
		CreatedAt: meta.Timestamp,
	}, nil
}

// streamNameFromTopic converts a topic string into a valid JetStream stream name.
// NATS stream names cannot contain '.', '>', or '*'.
func streamNameFromTopic(topic string) string {
	name := strings.NewReplacer(
		".", "_",
		">", "_",
		"*", "_",
		" ", "_",
	).Replace(topic)
	return name
}

// sanitizeName converts a consumer name into a valid NATS durable name.
func sanitizeName(name string) string {
	return strings.NewReplacer(
		".", "_",
		">", "_",
		"*", "_",
		" ", "_",
	).Replace(name)
}
