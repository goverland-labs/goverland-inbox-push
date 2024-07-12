package sender

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pevents "github.com/goverland-labs/goverland-platform-events/events/inbox"
	client "github.com/goverland-labs/goverland-platform-events/pkg/natsclient"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/goverland-labs/inbox-push/internal/config"
	"github.com/goverland-labs/inbox-push/internal/metrics"
)

const (
	maxPendingElements = 100
	rateLimit          = 500 * client.KiB
	executionTtl       = time.Minute
)

type PushManipulator interface {
	MarkAsClicked(id uuid.UUID) error
	ProcessFeedItem(ctx context.Context, item Item) error
}

type closable interface {
	Close() error
}

type Consumer struct {
	conn      *nats.Conn
	service   PushManipulator
	consumers []closable
}

func NewConsumer(nc *nats.Conn, s *Service) (*Consumer, error) {
	c := &Consumer{
		conn:      nc,
		service:   s,
		consumers: make([]closable, 0),
	}

	return c, nil
}

func (c *Consumer) clickHandler() pevents.PushClickHandler {
	return func(payload pevents.PushClickPayload) error {
		var err error
		defer func(start time.Time) {
			metricHandleHistogram.
				WithLabelValues("click_push", metrics.ErrLabelValue(err)).
				Observe(time.Since(start).Seconds())
		}(time.Now())

		err = c.service.MarkAsClicked(payload.ID)

		collectStats("mark", "clicked", err)

		return err
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	group := config.GenerateGroupName("send_push")
	opts := []client.ConsumerOpt{
		client.WithRateLimit(rateLimit),
		client.WithMaxAckPending(maxPendingElements),
		client.WithAckWait(executionTtl),
	}

	clicked, err := client.NewConsumer(ctx, c.conn, group, pevents.SubjectPushClicked, c.clickHandler(), opts...)
	if err != nil {
		return fmt.Errorf("consume for %s/%s: %w", group, pevents.SubjectPushClicked, err)
	}
	feed, err := client.NewConsumer(ctx, c.conn, group, pevents.SubjectFeedUpdated, c.handleFeed(), opts...)
	if err != nil {
		return fmt.Errorf("consume for %s/%s: %w", group, pevents.SubjectFeedUpdated, err)
	}

	c.consumers = append(c.consumers, clicked, feed)

	log.Info().Msg("sender consumers is started")

	// todo: handle correct stopping the consumer by context
	<-ctx.Done()
	return c.stop()
}

func (c *Consumer) stop() error {
	for _, cs := range c.consumers {
		if err := cs.Close(); err != nil {
			log.Error().Err(err).Msg("unable to close sender consumer")
		}
	}

	return nil
}
