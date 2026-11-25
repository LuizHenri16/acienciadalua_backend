package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	appmiddleware "github.com/acienciadalua/api/internal/middleware"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminStoreStub struct {
	setting []byte
}

func (f *adminStoreStub) ListAdminSales(context.Context) ([]db.ListAdminSalesRow, error) {
	return []db.ListAdminSalesRow{{
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
	}}, nil
}

func (f *adminStoreStub) ListAdminPendingIntents(context.Context) ([]db.ListAdminPendingIntentsRow, error) {
	return nil, nil
}

func (f *adminStoreStub) CountAdminPendingSales(context.Context) (int64, error) {
	return 1, nil
}

func (f *adminStoreStub) ListAdminCustomers(context.Context) ([]db.ListAdminCustomersRow, error) {
	return []db.ListAdminCustomersRow{{
		ID:        "customer-1",
		Name:      "Ana",
		Email:     "ana@example.test",
		CreatedAt: types.NewTimestamp(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)),
	}}, nil
}

func (f *adminStoreStub) ListAdminCustomerPurchases(context.Context, []string) ([]db.ListAdminCustomerPurchasesRow, error) {
	return []db.ListAdminCustomerPurchasesRow{{
		CustomerId:      "customer-1",
		SaleId:          "sale-1",
		ProductId:       "product-1",
		ProductTitle:    "Apostila",
		ProductCategory: "STUDENT",
		AmountPaid:      types.MustDecimal("19.9"),
		PurchasedAt:     types.NewTimestamp(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)),
	}}, nil
}

func (f *adminStoreStub) CountAdminSalesByProduct(context.Context) ([]db.CountAdminSalesByProductRow, error) {
	return []db.CountAdminSalesByProductRow{{ProductId: "product-1", Count: 3}}, nil
}

func (f *adminStoreStub) CountAdminCustomersAll(context.Context) (int64, error) {
	return 1, nil
}

func (f *adminStoreStub) AdminProductCounts(context.Context) (db.AdminProductCountsRow, error) {
	return db.AdminProductCountsRow{Total: 2, Active: 1}, nil
}

func (f *adminStoreStub) AdminPeriodStats(context.Context, db.AdminPeriodStatsParams) (db.AdminPeriodStatsRow, error) {
	return db.AdminPeriodStatsRow{
		TotalSalesCount: 2,
		TotalRevenue:    types.MustDecimal("49.8"),
		ApprovedCount:   2,
	}, nil
}

func (f *adminStoreStub) AdminDailySeries(context.Context, db.AdminDailySeriesParams) ([]db.AdminDailySeriesRow, error) {
	return []db.AdminDailySeriesRow{}, nil
}

func (f *adminStoreStub) AdminMonthlySeries(context.Context, db.AdminMonthlySeriesParams) ([]db.AdminMonthlySeriesRow, error) {
	return []db.AdminMonthlySeriesRow{}, nil
}

func (f *adminStoreStub) AdminTopProducts(context.Context, db.AdminTopProductsParams) ([]db.AdminTopProductsRow, error) {
	return []db.AdminTopProductsRow{}, nil
}

func (f *adminStoreStub) ListAdminActivity(context.Context) ([]db.ListAdminActivityRow, error) {
	return []db.ListAdminActivityRow{}, nil
}

func (f *adminStoreStub) AdminLastWebhookCheck(context.Context) (types.Timestamp, error) {
	return types.Timestamp{}, nil
}

func (f *adminStoreStub) GetAdminSetting(context.Context, string) ([]byte, error) {
	return f.setting, nil
}

func (f *adminStoreStub) UpsertAdminSetting(_ context.Context, arg db.UpsertAdminSettingParams) ([]byte, error) {
	f.setting = arg.Data
	return arg.Data, nil
}

const adminTestSecret = "admin-test-secret"

func adminTestRouter(store *adminStoreStub) http.Handler {
	cfg := &config.Config{MPWebhookURL: "https://example.test/payment/webhook"}
	handlers := &Handlers{admin: services.NewAdminService(cfg, store)}
	router := chi.NewRouter()
	router.Route("/admin", func(r chi.Router) {
		r.Use(appmiddleware.RequireAdmin(adminTestSecret))
		r.Get("/dashboard", handlers.GetAdminDashboard)
		r.Get("/sales", handlers.ListAdminSales)
		r.Get("/customers", handlers.ListAdminCustomers)
		r.Get("/products/sales-count", handlers.GetProductSalesCount)
		r.Get("/settings", handlers.GetAdminSettings)
		r.Put("/settings", handlers.PutAdminSettings)
	})
	return router
}

func adminTestToken(t *testing.T) string {
	t.Helper()
	token, err := utils.SignAdminToken("admin-1", "admin@example.test", "admin", adminTestSecret, time.Hour)
	require.NoError(t, err)
	return token
}

func adminTestRequest(t *testing.T, router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminRoutesRequireToken(t *testing.T) {
	router := adminTestRouter(&adminStoreStub{})
	for _, path := range []string{
		"/admin/dashboard?period=7d",
		"/admin/sales",
		"/admin/customers",
		"/admin/products/sales-count",
		"/admin/settings",
	} {
		response := adminTestRequest(t, router, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusUnauthorized, response.Code, path)
		assert.Contains(t, response.Body.String(), `"statusCode":401`)
	}
}

func TestAdminDashboardValidation(t *testing.T) {
	router := adminTestRouter(&adminStoreStub{})
	response := adminTestRequest(t, router, http.MethodGet, "/admin/dashboard?period=15d", "", adminTestToken(t))
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "period must be one of the following values")

	response = adminTestRequest(t, router, http.MethodGet, "/admin/dashboard?period=7d", "", adminTestToken(t))
	require.Equal(t, http.StatusOK, response.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	for _, key := range []string{"kpis", "periodSummary", "revenueSeries", "topProducts", "recentSales", "recentActivity", "pendingSalesCount", "webhookStatus"} {
		assert.Contains(t, payload, key)
	}
	assert.NotContains(t, payload, "isDemo")
	assert.NotContains(t, payload, "productsLive")
	assert.Equal(t, []any{}, payload["revenueSeries"])
}

func TestAdminListEndpointsReturnDirectArrays(t *testing.T) {
	router := adminTestRouter(&adminStoreStub{})
	token := adminTestToken(t)

	response := adminTestRequest(t, router, http.MethodGet, "/admin/sales", "", token)
	require.Equal(t, http.StatusOK, response.Code)
	var sales []map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &sales))
	require.Len(t, sales, 1)
	assert.Equal(t, "sale-1", sales[0]["id"])
	assert.Equal(t, "19.9", sales[0]["amount"])
	assert.Equal(t, "checkout_pro", sales[0]["paymentMethod"])
	assert.Equal(t, "approved", sales[0]["status"])

	response = adminTestRequest(t, router, http.MethodGet, "/admin/customers", "", token)
	require.Equal(t, http.StatusOK, response.Code)
	var customers []map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &customers))
	require.Len(t, customers, 1)
	assert.Equal(t, "customer-1", customers[0]["id"])
	purchases, ok := customers[0]["purchases"].([]any)
	require.True(t, ok)
	require.Len(t, purchases, 1)
	assert.Equal(t, "sale-1", purchases[0].(map[string]any)["saleId"])

	response = adminTestRequest(t, router, http.MethodGet, "/admin/products/sales-count", "", token)
	require.Equal(t, http.StatusOK, response.Code)
	var counts map[string]int
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &counts))
	assert.Equal(t, 3, counts["product-1"])
}

func TestAdminSettingsEndpoints(t *testing.T) {
	router := adminTestRouter(&adminStoreStub{})
	token := adminTestToken(t)

	response := adminTestRequest(t, router, http.MethodGet, "/admin/settings", "", token)
	require.Equal(t, http.StatusOK, response.Code)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &settings))
	assert.Equal(t, "A Ciência da Lua", settings["storeName"])

	body := `{"storeName":"Nova Loja","supportEmail":"suporte@example.test","storefrontEnabled":false,"autoReleaseOnApproval":true,"notifyOnNewSale":true,"notifyOnPendingPix":true,"mercadoPagoMode":"sandbox","webhookUrl":"https://example.test/hook"}`
	response = adminTestRequest(t, router, http.MethodPut, "/admin/settings", body, token)
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &settings))
	assert.Equal(t, "Nova Loja", settings["storeName"])
	assert.Equal(t, "sandbox", settings["mercadoPagoMode"])

	response = adminTestRequest(t, router, http.MethodPut, "/admin/settings", body+`,"unexpected":true}`, "")
	assert.Equal(t, http.StatusUnauthorized, response.Code)

	response = adminTestRequest(t, router, http.MethodPut, "/admin/settings", `{"storeName":"Loja","supportEmail":"suporte@example.test","mercadoPagoMode":"production","unexpected":true}`, token)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "property unexpected should not exist")

	response = adminTestRequest(t, router, http.MethodPut, "/admin/settings", `{"storeName":"Loja","supportEmail":"nao-email","mercadoPagoMode":"production"}`, token)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "supportEmail must be an email")
}
