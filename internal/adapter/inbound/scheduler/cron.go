package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// DueProcessor is satisfied by app.ScheduledService.
type DueProcessor interface {
	ProcessDue(ctx context.Context) error
}

// Scheduler evaluates due scheduled transactions every minute.
type Scheduler struct {
	processor DueProcessor
	logger    *slog.Logger
}

func New(processor DueProcessor, logger *slog.Logger) *Scheduler {
	return &Scheduler{processor: processor, logger: logger}
}

// Run starts the scheduler loop. It blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	s.logger.Info("scheduler started")
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			if err := s.processor.ProcessDue(ctx); err != nil {
				s.logger.Error("scheduler process due", "err", err)
			}
		}
	}
}
