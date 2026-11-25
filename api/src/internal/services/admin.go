package services

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// adminStore is the narrow set of analytics queries used by AdminService.
// *db.Queries implements it; tests supply an in-memory fake.
type adminStore interface {
	ListAdminSales(context.Context) ([]db.ListAdminSalesRow, error)
	ListAdminPendingIntents(context.Context) ([]db.ListAdminPendingIntentsRow, error)
	CountAdminPendingSales(context.Context) (int64, error)
	ListAdminCustomers(context.Context) ([]db.ListAdminCustomersRow, error)
	ListAdminCustomerPurchases(context.Context, []string) ([]db.ListAdminCustomerPurchasesRow, error)
	CountAdminSalesByProduct(context.Context) ([]db.CountAdminSalesByProductRow, error)
	CountAdminCustomersAll(context.Context) (int64, error)
	AdminProductCounts(context.Context) (db.AdminProductCountsRow, error)
	AdminPeriodStats(context.Context, db.AdminPeriodStatsParams) (db.AdminPeriodStatsRow, error)
	AdminDailySeries(context.Context, db.AdminDailySeriesParams) ([]db.AdminDailySeriesRow, error)
	AdminMonthlySeries(context.Context, db.AdminMonthlySeriesParams) ([]db.AdminMonthlySeriesRow, error)
	AdminTopProducts(context.Context, db.AdminTopProductsParams) ([]db.AdminTopProductsRow, error)
	ListAdminActivity(context.Context) ([]db.ListAdminActivityRow, error)
	AdminLastWebhookCheck(context.Context) (types.Timestamp, error)
	GetAdminSetting(context.Context, string) ([]byte, error)
	UpsertAdminSetting(context.Context, db.UpsertAdminSettingParams) ([]byte, error)
}

const (
	adminSettingKey = "default"
	adminListLimit  = 200
)

// AdminService implements the read-only analytics endpoints and the single
// admin settings document.
//
// Payment data is approximated: a linked PaymentIntent means `pix`, a Purchase
// without an intent means `checkout_pro` (there is no ledger for that flow).
// Rejected/refunded states outside PIX are not persisted today, so those
// statuses cannot appear for checkout_pro sales.
type AdminService struct {
	cfg *config.Config
	q   adminStore
	now func() time.Time
}

func NewAdminService(cfg *config.Config, q adminStore) *AdminService {
	return &AdminService{cfg: cfg, q: q, now: time.Now}
}

type adminWindow struct {
	from types.Timestamp
	to   types.Timestamp
}

// GetDashboard assembles the dashboard payload for one of the supported periods.
func (s *AdminService) GetDashboard(ctx context.Context, period string) (dto.AdminDashboardDTO, error) {
	current, previous, label, err := s.windowsForPeriod(period)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}

	now := s.now().UTC()
	month, previousMonth := monthlyWindows(now)

	stats, err := s.q.AdminPeriodStats(ctx, db.AdminPeriodStatsParams{From: current.from, To: current.to})
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	previousStats, err := s.q.AdminPeriodStats(ctx, db.AdminPeriodStatsParams{From: previous.from, To: previous.to})
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	monthStats, err := s.q.AdminPeriodStats(ctx, db.AdminPeriodStatsParams{From: month.from, To: month.to})
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	previousMonthStats, err := s.q.AdminPeriodStats(ctx, db.AdminPeriodStatsParams{From: previousMonth.from, To: previousMonth.to})
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}

	customersCount, err := s.q.CountAdminCustomersAll(ctx)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	productCounts, err := s.q.AdminProductCounts(ctx)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	pendingCount, err := s.q.CountAdminPendingSales(ctx)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}

	series, err := s.revenueSeries(ctx, period, current)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	topProducts, err := s.topProducts(ctx, current)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	sales, err := s.ListSales(ctx)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	activity, err := s.recentActivity(ctx, now)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}
	webhook, err := s.webhookStatus(ctx, now)
	if err != nil {
		return dto.AdminDashboardDTO{}, err
	}

	recentSales := sales
	if len(recentSales) > 6 {
		recentSales = recentSales[:6]
	}

	return dto.AdminDashboardDTO{
		KPIs: dto.DashboardKPIs{
			MonthlySalesCount:         monthStats.TotalSalesCount,
			PreviousMonthlySalesCount: previousMonthStats.TotalSalesCount,
			MonthlySalesDeltaPct:      ratioDelta(float64(monthStats.TotalSalesCount), float64(previousMonthStats.TotalSalesCount)),
			MonthlyRevenue:            monthStats.TotalRevenue,
			PreviousMonthlyRevenue:    previousMonthStats.TotalRevenue,
			MonthlyRevenueDeltaPct:    decimalDelta(monthStats.TotalRevenue, previousMonthStats.TotalRevenue),
			ActiveProductsCount:       productCounts.Active,
			TotalProductsCount:        productCounts.Total,
			TotalCustomersCount:       customersCount,
			NewCustomersMonthCount:    monthStats.NewCustomersCount,
		},
		PeriodSummary:     periodSummary(stats, previousStats, label),
		RevenueSeries:     series,
		TopProducts:       topProducts,
		RecentSales:       recentSales,
		RecentActivity:    activity,
		PendingSalesCount: pendingCount,
		WebhookStatus:     webhook,
	}, nil
}

// ListSales returns purchases and unlinked payment intents as one list, newest
// first, capped at 200 rows.
func (s *AdminService) ListSales(ctx context.Context) ([]dto.AdminSale, error) {
	purchases, err := s.q.ListAdminSales(ctx)
	if err != nil {
		return nil, err
	}
	intents, err := s.q.ListAdminPendingIntents(ctx)
	if err != nil {
		return nil, err
	}

	sales := make([]dto.AdminSale, 0, len(purchases)+len(intents))
	for _, row := range purchases {
		sales = append(sales, dto.AdminSale{
			ID:                row.SaleId,
			ExternalPaymentID: row.ExternalPaymentId,
			CustomerID:        row.CustomerId,
			CustomerName:      row.CustomerName,
			CustomerEmail:     row.CustomerEmail,
			ProductID:         row.ProductId,
			ProductTitle:      row.ProductTitle,
			ProductCategory:   row.ProductCategory,
			Amount:            row.AmountPaid,
			PaymentMethod:     paymentMethod(row.IntentStatus != nil),
			Status:            mapAdminStatus(row.IntentStatus),
			StatusDetail:      nonEmpty(row.IntentStatusDetail),
			AccessGranted:     true,
			CreatedAt:         row.PurchasedAt,
		})
	}
	for _, row := range intents {
		status := row.IntentStatus
		sales = append(sales, dto.AdminSale{
			ID:                row.IntentId,
			ExternalPaymentID: row.ExternalPaymentId,
			CustomerID:        row.CustomerId,
			CustomerName:      row.CustomerName,
			CustomerEmail:     row.CustomerEmail,
			ProductID:         row.ProductId,
			ProductTitle:      row.ProductTitle,
			ProductCategory:   row.ProductCategory,
			Amount:            row.Amount,
			PaymentMethod:     paymentMethod(true),
			Status:            mapAdminStatus(&status),
			StatusDetail:      nonEmpty(&row.IntentStatusDetail),
			AccessGranted:     false,
			CreatedAt:         row.IntentCreatedAt,
		})
	}

	sort.Slice(sales, func(i, j int) bool {
		if sales[i].CreatedAt.AsTime().Equal(sales[j].CreatedAt.AsTime()) {
			return sales[i].ID > sales[j].ID
		}
		return sales[i].CreatedAt.AsTime().After(sales[j].CreatedAt.AsTime())
	})
	if len(sales) > adminListLimit {
		sales = sales[:adminListLimit]
	}
	return sales, nil
}

// ListCustomers returns customers (newest first) with their nested purchases.
func (s *AdminService) ListCustomers(ctx context.Context) ([]dto.AdminCustomer, error) {
	rows, err := s.q.ListAdminCustomers(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	purchaseRows, err := s.q.ListAdminCustomerPurchases(ctx, ids)
	if err != nil {
		return nil, err
	}

	byCustomer := make(map[string][]dto.AdminCustomerPurchase, len(rows))
	for _, row := range purchaseRows {
		status := dto.AdminCustomerPurchase{
			SaleID:          row.SaleId,
			ProductID:       row.ProductId,
			ProductTitle:    row.ProductTitle,
			ProductCategory: row.ProductCategory,
			Amount:          row.AmountPaid,
			Status:          mapAdminStatus(row.IntentStatus),
			PaymentMethod:   paymentMethod(row.IntentStatus != nil),
			PurchasedAt:     row.PurchasedAt,
		}
		byCustomer[row.CustomerId] = append(byCustomer[row.CustomerId], status)
	}

	customers := make([]dto.AdminCustomer, 0, len(rows))
	for _, row := range rows {
		purchases := byCustomer[row.ID]
		if purchases == nil {
			purchases = []dto.AdminCustomerPurchase{}
		}
		var lastPurchaseAt *types.Timestamp
		if !row.LastPurchaseAt.IsZero() {
			timestamp := row.LastPurchaseAt
			lastPurchaseAt = &timestamp
		}
		customers = append(customers, dto.AdminCustomer{
			ID:             row.ID,
			Name:           row.Name,
			Email:          row.Email,
			CreatedAt:      row.CreatedAt,
			PurchasesCount: row.PurchasesCount,
			TotalSpent:     row.TotalSpent,
			LastPurchaseAt: lastPurchaseAt,
			Purchases:      purchases,
		})
	}
	return customers, nil
}

// SalesCountByProduct returns the all-time number of sales per product.
func (s *AdminService) SalesCountByProduct(ctx context.Context) (map[string]int64, error) {
	rows, err := s.q.CountAdminSalesByProduct(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.ProductId] = row.Count
	}
	return counts, nil
}

// GetSettings falls back to defaults when the single settings row is absent.
func (s *AdminService) GetSettings(ctx context.Context) (dto.AdminSettings, error) {
	data, err := s.q.GetAdminSetting(ctx, adminSettingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.defaultSettings(), nil
	}
	if err != nil {
		return dto.AdminSettings{}, err
	}
	return decodeAdminSettings(data, s.defaultSettings())
}

// UpdateSettings replaces the single settings document.
func (s *AdminService) UpdateSettings(ctx context.Context, input dto.UpdateAdminSettingsRequest) (dto.AdminSettings, error) {
	settings := dto.AdminSettings{
		StoreName:             input.StoreName,
		SupportEmail:          input.SupportEmail,
		StorefrontEnabled:     input.StorefrontEnabled,
		AutoReleaseOnApproval: input.AutoReleaseOnApproval,
		NotifyOnNewSale:       input.NotifyOnNewSale,
		NotifyOnPendingPix:    input.NotifyOnPendingPix,
		MercadoPagoMode:       input.MercadoPagoMode,
		WebhookURL:            input.WebhookURL,
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return dto.AdminSettings{}, err
	}
	stored, err := s.q.UpsertAdminSetting(ctx, db.UpsertAdminSettingParams{
		Key:  adminSettingKey,
		Data: data,
	})
	if err != nil {
		return dto.AdminSettings{}, err
	}
	return decodeAdminSettings(stored, settings)
}

func (s *AdminService) defaultSettings() dto.AdminSettings {
	return dto.AdminSettings{
		StoreName:             "A Ciência da Lua",
		SupportEmail:          "contato@acienciadalua.com.br",
		StorefrontEnabled:     true,
		AutoReleaseOnApproval: true,
		NotifyOnNewSale:       true,
		NotifyOnPendingPix:    true,
		MercadoPagoMode:       "production",
		WebhookURL:            s.cfg.MPWebhookURL,
	}
}

func decodeAdminSettings(data []byte, fallback dto.AdminSettings) (dto.AdminSettings, error) {
	settings := fallback
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return dto.AdminSettings{}, err
	}
	return settings, nil
}

func (s *AdminService) windowsForPeriod(period string) (adminWindow, adminWindow, string, error) {
	now := s.now().UTC()
	var current, previous adminWindow
	var label string

	switch period {
	case dto.Period7D, dto.Period30D, dto.Period90D:
		days := map[string]int{dto.Period7D: 7, dto.Period30D: 30, dto.Period90D: 90}[period]
		current = adminWindow{from: types.NewTimestamp(now.AddDate(0, 0, -days)), to: types.NewTimestamp(now)}
		previous = adminWindow{from: types.NewTimestamp(now.AddDate(0, 0, -2*days)), to: current.from}
		label = "vs. " + periodLabelDays(days)
	case dto.Period1Y:
		current = adminWindow{from: types.NewTimestamp(now.AddDate(-1, 0, 0)), to: types.NewTimestamp(now)}
		previous = adminWindow{from: types.NewTimestamp(now.AddDate(-2, 0, 0)), to: current.from}
		label = "vs. ano anterior"
	default:
		return adminWindow{}, adminWindow{}, "", utils.BadRequest(periodValidationMessage)
	}

	return current, previous, label, nil
}

const periodValidationMessage = "period must be one of the following values: 7d, 30d, 90d, 1y"

func periodLabelDays(days int) string {
	switch days {
	case 7:
		return "7 dias anteriores"
	case 30:
		return "30 dias anteriores"
	default:
		return "90 dias anteriores"
	}
}

func monthlyWindows(now time.Time) (adminWindow, adminWindow) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	previousStart := monthStart.AddDate(0, -1, 0)
	return adminWindow{from: types.NewTimestamp(monthStart), to: types.NewTimestamp(now)},
		adminWindow{from: types.NewTimestamp(previousStart), to: types.NewTimestamp(monthStart)}
}

func (s *AdminService) revenueSeries(ctx context.Context, period string, window adminWindow) ([]dto.RevenueSeriesPoint, error) {
	points := []dto.RevenueSeriesPoint{}
	if period == dto.Period1Y {
		rows, err := s.q.AdminMonthlySeries(ctx, db.AdminMonthlySeriesParams{From: window.from, To: window.to})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			bucket := row.Bucket.AsTime().UTC()
			points = append(points, dto.RevenueSeriesPoint{
				Date:         bucket.Format("2006-01"),
				Label:        portugueseMonth(bucket.Month()),
				Revenue:      row.Revenue,
				SalesCount:   row.SalesCount,
				NewCustomers: row.NewCustomers,
			})
		}
		return points, nil
	}

	rows, err := s.q.AdminDailySeries(ctx, db.AdminDailySeriesParams{From: window.from, To: window.to})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		bucket := row.Bucket.AsTime().UTC()
		points = append(points, dto.RevenueSeriesPoint{
			Date:         bucket.Format("2006-01-02"),
			Label:        bucket.Format("02/01"),
			Revenue:      row.Revenue,
			SalesCount:   row.SalesCount,
			NewCustomers: row.NewCustomers,
		})
	}
	return points, nil
}

func portugueseMonth(month time.Month) string {
	names := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
	return names[month-1]
}

func (s *AdminService) topProducts(ctx context.Context, window adminWindow) ([]dto.TopProductItem, error) {
	rows, err := s.q.AdminTopProducts(ctx, db.AdminTopProductsParams{From: window.from, To: window.to})
	if err != nil {
		return nil, err
	}
	items := make([]dto.TopProductItem, 0, len(rows))
	tones := []string{"teal", "lilac", "amber", "navy", "rose"}
	for index, row := range rows {
		tone := tones[index%len(tones)]
		items = append(items, dto.TopProductItem{
			ID:         row.ID,
			Title:      row.Title,
			Category:   row.ProductCategory,
			SalesCount: row.SalesCount,
			Revenue:    row.Revenue,
			Price:      row.Price,
			Tone:       &tone,
		})
	}
	return items, nil
}

func periodSummary(current, previous db.AdminPeriodStatsRow, label string) dto.PeriodSummaryMetrics {
	averageTicket := average(current.TotalRevenue, current.TotalSalesCount)
	previousAverageTicket := average(previous.TotalRevenue, previous.TotalSalesCount)
	return dto.PeriodSummaryMetrics{
		TotalRevenue:          current.TotalRevenue,
		RevenueDeltaPct:       decimalDelta(current.TotalRevenue, previous.TotalRevenue),
		ComparisonLabel:       label,
		AverageTicket:         averageTicket,
		AverageTicketDeltaPct: decimalDelta(averageTicket, previousAverageTicket),
		ApprovalRate:          percentage(current.ApprovedCount, current.TotalSalesCount),
		ApprovalRateDeltaPp:   percentage(current.ApprovedCount, current.TotalSalesCount) - percentage(previous.ApprovedCount, previous.TotalSalesCount),
		RefundsCount:          current.RefundsCount,
		RefundsDelta:          current.RefundsCount - previous.RefundsCount,
		TotalSalesCount:       current.TotalSalesCount,
		NewCustomersCount:     current.NewCustomersCount,
	}
}

func average(total types.Decimal, count int64) types.Decimal {
	if count == 0 {
		return types.Decimal{Decimal: decimal.Zero}
	}
	return types.Decimal{Decimal: total.Decimal.Div(decimal.NewFromInt(count))}
}

func percentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func ratioDelta(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / previous * 100
}

func decimalDelta(current, previous types.Decimal) float64 {
	currentFloat, _ := current.Float64()
	previousFloat, _ := previous.Float64()
	return ratioDelta(currentFloat, previousFloat)
}

func (s *AdminService) recentActivity(ctx context.Context, now time.Time) ([]dto.AdminActivity, error) {
	rows, err := s.q.ListAdminActivity(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AdminActivity, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.AdminActivity{
			ID:           row.ID,
			Type:         row.Type,
			Title:        row.Title,
			Description:  row.Description,
			CreatedAt:    row.CreatedAt,
			RelativeTime: relativeTime(now, row.CreatedAt.AsTime()),
		})
	}
	return items, nil
}

func relativeTime(now, at time.Time) string {
	diff := now.Sub(at.UTC())
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes < 1 {
			minutes = 1
		}
		return "há " + itoa(minutes) + " min"
	case diff < 24*time.Hour:
		return "há " + itoa(int(diff.Hours())) + " h"
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "há 1 dia"
		}
		return "há " + itoa(days) + " dias"
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "há 1 mês"
		}
		return "há " + itoa(months) + " meses"
	default:
		years := int(diff.Hours() / (24 * 365))
		if years == 1 {
			return "há 1 ano"
		}
		return "há " + itoa(years) + " anos"
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		digits[index] = '-'
	}
	return string(digits[index:])
}

func (s *AdminService) webhookStatus(ctx context.Context, now time.Time) (dto.WebhookStatus, error) {
	last, err := s.q.AdminLastWebhookCheck(ctx)
	if err != nil {
		return dto.WebhookStatus{}, err
	}
	status := dto.WebhookStatus{
		Connected:         s.cfg.MPWebhookURL != "",
		LastEventRelative: "sem eventos recentes",
	}
	if !last.IsZero() {
		status.LastEventRelative = relativeTime(now, last.AsTime())
	}
	return status, nil
}

// mapAdminStatus normalizes raw payment statuses to the five UI statuses.
// A Purchase without an intent is treated as approved because the access
// ledger only holds successful purchases.
func mapAdminStatus(intentStatus *string) string {
	if intentStatus == nil {
		return dto.SaleStatusApproved
	}
	switch *intentStatus {
	case "approved":
		return dto.SaleStatusApproved
	case "pending", "in_process", "authorized", "in_mediation":
		return dto.SaleStatusPending
	case "rejected", "cancelled", "error":
		return dto.SaleStatusRejected
	case "expired":
		return dto.SaleStatusCancelled
	case "refunded", "charged_back":
		return dto.SaleStatusRefunded
	default:
		return dto.SaleStatusApproved
	}
}

// paymentMethod is an approximation: an existing intent means PIX, while a
// Purchase without one comes from checkout_pro. credit_card is never inferred.
func paymentMethod(hasIntent bool) string {
	if hasIntent {
		return dto.PaymentMethodPix
	}
	return dto.PaymentMethodCheckoutPro
}

func nonEmpty(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}
