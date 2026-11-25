package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminStoreFake struct {
	sales             []db.ListAdminSalesRow
	intents           []db.ListAdminPendingIntentsRow
	pending           int64
	customers         []db.ListAdminCustomersRow
	customerPurchases []db.ListAdminCustomerPurchasesRow
	salesByProduct    []db.CountAdminSalesByProductRow
	customersAll      int64
	productCounts     db.AdminProductCountsRow
	stats             []db.AdminPeriodStatsRow
	statsParams       []db.AdminPeriodStatsParams
	daily             []db.AdminDailySeriesRow
	monthly           []db.AdminMonthlySeriesRow
	top               []db.AdminTopProductsRow
	activity          []db.ListAdminActivityRow
	webhook           types.Timestamp
	setting           []byte
}

func (f *adminStoreFake) ListAdminSales(context.Context) ([]db.ListAdminSalesRow, error) {
	return f.sales, nil
}

func (f *adminStoreFake) ListAdminPendingIntents(context.Context) ([]db.ListAdminPendingIntentsRow, error) {
	return f.intents, nil
}

func (f *adminStoreFake) CountAdminPendingSales(context.Context) (int64, error) {
	return f.pending, nil
}

func (f *adminStoreFake) ListAdminCustomers(context.Context) ([]db.ListAdminCustomersRow, error) {
	return f.customers, nil
}

func (f *adminStoreFake) ListAdminCustomerPurchases(context.Context, []string) ([]db.ListAdminCustomerPurchasesRow, error) {
	return f.customerPurchases, nil
}

func (f *adminStoreFake) CountAdminSalesByProduct(context.Context) ([]db.CountAdminSalesByProductRow, error) {
	return f.salesByProduct, nil
}

func (f *adminStoreFake) CountAdminCustomersAll(context.Context) (int64, error) {
	return f.customersAll, nil
}

func (f *adminStoreFake) AdminProductCounts(context.Context) (db.AdminProductCountsRow, error) {
	return f.productCounts, nil
}

func (f *adminStoreFake) AdminPeriodStats(_ context.Context, arg db.AdminPeriodStatsParams) (db.AdminPeriodStatsRow, error) {
	f.statsParams = append(f.statsParams, arg)
	if len(f.stats) == 0 {
		return db.AdminPeriodStatsRow{}, nil
	}
	row := f.stats[0]
	f.stats = f.stats[1:]
	return row, nil
}

func (f *adminStoreFake) AdminDailySeries(context.Context, db.AdminDailySeriesParams) ([]db.AdminDailySeriesRow, error) {
	return f.daily, nil
}

func (f *adminStoreFake) AdminMonthlySeries(context.Context, db.AdminMonthlySeriesParams) ([]db.AdminMonthlySeriesRow, error) {
	return f.monthly, nil
}

func (f *adminStoreFake) AdminTopProducts(context.Context, db.AdminTopProductsParams) ([]db.AdminTopProductsRow, error) {
	return f.top, nil
}

func (f *adminStoreFake) ListAdminActivity(context.Context) ([]db.ListAdminActivityRow, error) {
	return f.activity, nil
}

func (f *adminStoreFake) AdminLastWebhookCheck(context.Context) (types.Timestamp, error) {
	return f.webhook, nil
}

func (f *adminStoreFake) GetAdminSetting(context.Context, string) ([]byte, error) {
	return f.setting, nil
}

func (f *adminStoreFake) UpsertAdminSetting(_ context.Context, arg db.UpsertAdminSettingParams) ([]byte, error) {
	return arg.Data, nil
}

func newAdminServiceForTest(store adminStore, now time.Time) *AdminService {
	service := NewAdminService(&config.Config{MPWebhookURL: "https://example.test/payment/webhook"}, store)
	service.now = func() time.Time { return now }
	return service
}

func TestAdminPeriodValidation(t *testing.T) {
	store := &adminStoreFake{}
	service := newAdminServiceForTest(store, time.Now())

	_, err := service.GetDashboard(context.Background(), "15d")
	require.Error(t, err)
	appErr, ok := utils.AsError(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	assert.Equal(t, periodValidationMessage, appErr.Message)

	for _, period := range []string{dto.Period7D, dto.Period30D, dto.Period90D, dto.Period1Y} {
		_, err := service.GetDashboard(context.Background(), period)
		assert.NoError(t, err, period)
	}
}

func TestAdminWindowsUseUTC(t *testing.T) {
	service := newAdminServiceForTest(&adminStoreFake{}, time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC))

	current, previous, label, err := service.windowsForPeriod(dto.Period7D)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-08T14:30:00.000Z", timestampJSON(t, current.from))
	assert.Equal(t, "2026-06-15T14:30:00.000Z", timestampJSON(t, current.to))
	assert.Equal(t, "2026-06-01T14:30:00.000Z", timestampJSON(t, previous.from))
	assert.Equal(t, "vs. 7 dias anteriores", label)

	current, previous, label, err = service.windowsForPeriod(dto.Period1Y)
	require.NoError(t, err)
	assert.Equal(t, "2025-06-15T14:30:00.000Z", timestampJSON(t, current.from))
	assert.Equal(t, "2024-06-15T14:30:00.000Z", timestampJSON(t, previous.from))
	assert.Equal(t, "vs. ano anterior", label)

	month, previousMonth := monthlyWindows(service.now().UTC())
	assert.Equal(t, "2026-06-01T00:00:00.000Z", timestampJSON(t, month.from))
	assert.Equal(t, "2026-05-01T00:00:00.000Z", timestampJSON(t, previousMonth.from))
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func timestampJSON(t *testing.T, value any) string {
	t.Helper()
	var result string
	require.NoError(t, json.Unmarshal([]byte(mustJSON(t, value)), &result))
	return result
}

func TestAdminDeltasWithZeroPrevious(t *testing.T) {
	current := db.AdminPeriodStatsRow{
		TotalSalesCount:   5,
		TotalRevenue:      types.MustDecimal("100.00"),
		ApprovedCount:     4,
		RefundsCount:      1,
		NewCustomersCount: 2,
	}

	summary := periodSummary(current, db.AdminPeriodStatsRow{}, "vs. 7 dias anteriores")
	assert.Equal(t, float64(0), summary.RevenueDeltaPct)
	assert.Equal(t, float64(0), summary.AverageTicketDeltaPct)
	assert.Equal(t, float64(80), summary.ApprovalRate)
	assert.Equal(t, float64(80), summary.ApprovalRateDeltaPp)
	assert.Equal(t, int64(1), summary.RefundsDelta)
	assert.Equal(t, "20", summary.AverageTicket.Decimal.String())
	assert.Equal(t, "100", summary.TotalRevenue.Decimal.String())

	assert.Equal(t, float64(0), ratioDelta(10, 0))
	assert.Equal(t, float64(0), decimalDelta(types.MustDecimal("10"), types.Decimal{}))
	assert.Equal(t, float64(50), ratioDelta(15, 10))
}

func TestAdminStatusMapping(t *testing.T) {
	status := func(value string) *string { return &value }
	cases := map[string]string{
		"approved":     dto.SaleStatusApproved,
		"pending":      dto.SaleStatusPending,
		"in_process":   dto.SaleStatusPending,
		"authorized":   dto.SaleStatusPending,
		"in_mediation": dto.SaleStatusPending,
		"rejected":     dto.SaleStatusRejected,
		"cancelled":    dto.SaleStatusRejected,
		"error":        dto.SaleStatusRejected,
		"expired":      dto.SaleStatusCancelled,
		"refunded":     dto.SaleStatusRefunded,
		"charged_back": dto.SaleStatusRefunded,
		"unknown":      dto.SaleStatusApproved,
	}
	for input, expected := range cases {
		assert.Equal(t, expected, mapAdminStatus(status(input)), input)
	}
	assert.Equal(t, dto.SaleStatusApproved, mapAdminStatus(nil))
	assert.Equal(t, dto.PaymentMethodPix, paymentMethod(true))
	assert.Equal(t, dto.PaymentMethodCheckoutPro, paymentMethod(false))
}

func TestAdminRevenueSeriesBucketsAreUTC(t *testing.T) {
	store := &adminStoreFake{
		daily: []db.AdminDailySeriesRow{
			{
				Bucket:       types.NewTimestamp(time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)),
				Revenue:      types.MustDecimal("49.9"),
				SalesCount:   2,
				NewCustomers: 1,
			},
			{
				Bucket:       types.NewTimestamp(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)),
				Revenue:      types.Decimal{},
				SalesCount:   0,
				NewCustomers: 0,
			},
		},
		monthly: []db.AdminMonthlySeriesRow{
			{
				Bucket:       types.NewTimestamp(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)),
				Revenue:      types.MustDecimal("99.8"),
				SalesCount:   4,
				NewCustomers: 3,
			},
			{
				Bucket:       types.NewTimestamp(time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)),
				Revenue:      types.Decimal{},
				SalesCount:   0,
				NewCustomers: 0,
			},
		},
	}
	service := newAdminServiceForTest(store, time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC))
	window := adminWindow{
		from: types.NewTimestamp(time.Date(2026, 6, 8, 14, 30, 0, 0, time.UTC)),
		to:   types.NewTimestamp(time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)),
	}

	daily, err := service.revenueSeries(context.Background(), dto.Period7D, window)
	require.NoError(t, err)
	require.Len(t, daily, 2)
	assert.Equal(t, "2026-06-14", daily[0].Date)
	assert.Equal(t, "14/06", daily[0].Label)
	assert.Equal(t, "49.9", daily[0].Revenue.Decimal.String())
	assert.Equal(t, "2026-06-15", daily[1].Date)
	assert.Equal(t, "15/06", daily[1].Label)
	assert.Equal(t, "0", daily[1].Revenue.Decimal.String())

	monthly, err := service.revenueSeries(context.Background(), dto.Period1Y, window)
	require.NoError(t, err)
	require.Len(t, monthly, 2)
	assert.Equal(t, "2026-05", monthly[0].Date)
	assert.Equal(t, "Mai", monthly[0].Label)
	assert.Equal(t, "2025-12", monthly[1].Date)
	assert.Equal(t, "Dez", monthly[1].Label)
}

func TestAdminEmptyArraysAreSerializedAsArrays(t *testing.T) {
	service := newAdminServiceForTest(&adminStoreFake{}, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC))

	dashboard, err := service.GetDashboard(context.Background(), dto.Period30D)
	require.NoError(t, err)
	require.NotNil(t, dashboard.RevenueSeries)
	require.NotNil(t, dashboard.TopProducts)
	require.NotNil(t, dashboard.RecentSales)
	require.NotNil(t, dashboard.RecentActivity)

	raw := mustJSON(t, dashboard)
	assert.Contains(t, raw, `"revenueSeries":[]`)
	assert.Contains(t, raw, `"topProducts":[]`)
	assert.Contains(t, raw, `"recentSales":[]`)
	assert.Contains(t, raw, `"recentActivity":[]`)
	assert.NotContains(t, raw, `null`)

	customers, err := service.ListCustomers(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, customers)

	sales, err := service.ListSales(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, sales)

	counts, err := service.SalesCountByProduct(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, counts)
}

func TestAdminRelativeTime(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "há 12 min", relativeTime(now, now.Add(-12*time.Minute)))
	assert.Equal(t, "há 1 h", relativeTime(now, now.Add(-time.Hour)))
	assert.Equal(t, "há 3 dias", relativeTime(now, now.AddDate(0, 0, -3)))
	assert.Equal(t, "sem eventos recentes", func() string {
		status, err := newAdminServiceForTest(&adminStoreFake{}, now).webhookStatus(context.Background(), now)
		require.NoError(t, err)
		return status.LastEventRelative
	}())
}

func TestAdminSettingsDefaultsAndUpdate(t *testing.T) {
	service := newAdminServiceForTest(&adminStoreFake{}, time.Now())

	settings, err := service.GetSettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "A Ciência da Lua", settings.StoreName)
	assert.Equal(t, "https://example.test/payment/webhook", settings.WebhookURL)

	updated, err := service.UpdateSettings(context.Background(), dto.UpdateAdminSettingsRequest{
		StoreName:       "Nova Loja",
		SupportEmail:    "suporte@example.test",
		MercadoPagoMode: "sandbox",
	})
	require.NoError(t, err)
	assert.Equal(t, "Nova Loja", updated.StoreName)
	assert.Equal(t, "sandbox", updated.MercadoPagoMode)
}

func TestAdminSalesMergeAndCustomerPurchases(t *testing.T) {
	store := &adminStoreFake{
		sales: []db.ListAdminSalesRow{{
			SaleId:            "sale-1",
			ExternalPaymentId: "MP-1",
			CustomerId:        "customer-1",
			CustomerName:      "Ana",
			CustomerEmail:     "ana@example.test",
			ProductId:         "product-1",
			ProductTitle:      "Apostila",
			ProductCategory:   "STUDENT",
			AmountPaid:        types.MustDecimal("19.9"),
			PurchasedAt:       types.NewTimestamp(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)),
		}},
		intents: []db.ListAdminPendingIntentsRow{{
			IntentId:           "intent-1",
			ExternalPaymentId:  "MP-2",
			CustomerId:         "customer-2",
			CustomerName:       "João",
			CustomerEmail:      "joao@example.test",
			ProductId:          "product-2",
			ProductTitle:       "Bingo",
			ProductCategory:    "TEACHER",
			Amount:             types.MustDecimal("34.9"),
			IntentStatus:       "expired",
			IntentStatusDetail: "expired",
			IntentCreatedAt:    types.NewTimestamp(time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)),
		}},
		customers: []db.ListAdminCustomersRow{{
			ID:        "customer-1",
			Name:      "Ana",
			Email:     "ana@example.test",
			CreatedAt: types.NewTimestamp(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)),
		}},
		customerPurchases: []db.ListAdminCustomerPurchasesRow{{
			CustomerId:      "customer-1",
			SaleId:          "sale-1",
			ProductId:       "product-1",
			ProductTitle:    "Apostila",
			ProductCategory: "STUDENT",
			AmountPaid:      types.MustDecimal("19.9"),
			PurchasedAt:     types.NewTimestamp(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)),
		}},
	}
	service := newAdminServiceForTest(store, time.Now())

	sales, err := service.ListSales(context.Background())
	require.NoError(t, err)
	require.Len(t, sales, 2)
	assert.Equal(t, "intent-1", sales[0].ID)
	assert.Equal(t, dto.SaleStatusCancelled, sales[0].Status)
	assert.False(t, sales[0].AccessGranted)
	assert.Equal(t, dto.PaymentMethodPix, sales[0].PaymentMethod)
	assert.Equal(t, "sale-1", sales[1].ID)
	assert.True(t, sales[1].AccessGranted)
	assert.Equal(t, dto.PaymentMethodCheckoutPro, sales[1].PaymentMethod)

	customers, err := service.ListCustomers(context.Background())
	require.NoError(t, err)
	require.Len(t, customers, 1)
	require.Len(t, customers[0].Purchases, 1)
	assert.Equal(t, "sale-1", customers[0].Purchases[0].SaleID)
	assert.Nil(t, customers[0].LastPurchaseAt)
}

func TestAdminStoreErrorIsPropagated(t *testing.T) {
	service := newAdminServiceForTest(&failingAdminStore{}, time.Now())
	_, err := service.ListSales(context.Background())
	assert.ErrorIs(t, err, errAdminStore)
}

var errAdminStore = errors.New("store unavailable")

type failingAdminStore struct{ adminStoreFake }

func (f *failingAdminStore) ListAdminSales(context.Context) ([]db.ListAdminSalesRow, error) {
	return nil, errAdminStore
}
