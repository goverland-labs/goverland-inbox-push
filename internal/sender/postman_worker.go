package sender

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	reSendDelay            = 5 * time.Minute
	reSendImmediatelyDelay = 5 * time.Minute
)

type PostmanWorker struct {
	service *Service
}

func NewPostmanWorker(s *Service) *PostmanWorker {
	return &PostmanWorker{
		service: s,
	}
}

func (w *PostmanWorker) StartRegular(ctx context.Context) error {
	for {
		start := time.Now()
		err := w.service.sendBatch(ctx)
		if err != nil {
			log.Error().Err(err).Msg("send batch")
		}

		log.Debug().Msgf("send batch completed: %v", time.Since(start))

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reSendDelay):
		}
	}
}

func (w *PostmanWorker) StartImmediately(ctx context.Context) error {
	for {
		err := w.service.sendVotingEndsSoon(ctx)
		if err != nil {
			log.Error().Err(err).Msg("send immediately")
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reSendImmediatelyDelay):
		}
	}
}
