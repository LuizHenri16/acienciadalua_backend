package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/acienciadalua/api/internal/services"
	"github.com/robfig/cron/v3"
)

// StartPaymentReconciliation schedules the payment reconciliation job. It
// refreshes pending PaymentIntents against Mercado Pago so an approved payment
// creates its Purchase even when the webhook is lost or delayed.
func StartPaymentReconciliation(
	payment *services.PaymentService,
	interval time.Duration,
	logger *slog.Logger,
) (*cron.Cron, error) {
	if interval <= 0 {
		interval = 2 * time.Minute
	}

	scheduler := cron.New()
	scheduler.Schedule(cron.Every(interval), cron.FuncJob(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := payment.ReconcilePending(ctx); err != nil {
			logger.Error("Falha na reconciliação de pagamentos", "error", err)
		}
	}))

	scheduler.Start()
	return scheduler, nil
}
