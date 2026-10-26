// Package jobs contains scheduled tasks, equivalent to @nestjs/schedule.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/robfig/cron/v3"
)

// StartTokenCleanup schedules the daily cleanup of expired/used magic-link
// tokens (AuthTokenExpiredCleanupService used @Cron(EVERY_DAY_AT_MIDNIGHT)).
func StartTokenCleanup(q *db.Queries, logger *slog.Logger) (*cron.Cron, error) {
	scheduler := cron.New()

	_, err := scheduler.AddFunc("@midnight", func() {
		logger.Info("Iniciando a limpeza de tokens (Magic Links) expirados ou usados...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		removed, err := q.DeleteExpiredOrUsedAuthTokens(ctx, types.NewTimestamp(time.Now()))
		if err != nil {
			logger.Error("Falha ao limpar tokens do banco de dados", "error", err)
			return
		}

		logger.Info("Limpeza concluída", "removed", removed)
	})
	if err != nil {
		return nil, err
	}

	scheduler.Start()
	return scheduler, nil
}
