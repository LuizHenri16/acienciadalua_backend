package dto

import (
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
)

// Period values accepted by GET /admin/dashboard.
const (
	Period7D  = "7d"
	Period30D = "30d"
	Period90D = "90d"
	Period1Y  = "1y"
)

// Sale statuses exposed by the admin panel.
const (
	SaleStatusApproved  = "approved"
	SaleStatusPending   = "pending"
	SaleStatusRejected  = "rejected"
	SaleStatusCancelled = "cancelled"
	SaleStatusRefunded  = "refunded"
)

// Payment methods exposed by the admin panel. See AdminService for the
// approximation used when no PaymentIntent is linked to a Purchase.
const (
	PaymentMethodPix         = "pix"
	PaymentMethodCreditCard  = "credit_card"
	PaymentMethodCheckoutPro = "checkout_pro"
)

// Activity types shown in the dashboard feed.
const (
	ActivitySaleApproved       = "sale_approved"
	ActivityCustomerRegistered = "customer_registered"
	ActivityPaymentPending     = "payment_pending"
	ActivityProductUpdated     = "product_updated"
	ActivitySaleRefunded       = "sale_refunded"
)

// DashboardQuery is the query string of GET /admin/dashboard.
type DashboardQuery struct {
	Period string `json:"period" validate:"required,oneof=7d 30d 90d 1y"`
}

var dashboardQueryMessages = utils.Messages{
	"period.required": "period must be one of the following values: 7d, 30d, 90d, 1y",
	"period.oneof":    "period must be one of the following values: 7d, 30d, 90d, 1y",
}

func (d DashboardQuery) Validate() []string { return utils.Validate(d, dashboardQueryMessages) }

// DashboardKPIs mirrors the monthly KPI cards of the admin dashboard.
type DashboardKPIs struct {
	MonthlySalesCount         int64         `json:"monthlySalesCount"`
	PreviousMonthlySalesCount int64         `json:"previousMonthlySalesCount"`
	MonthlySalesDeltaPct      float64       `json:"monthlySalesDeltaPct"`
	MonthlyRevenue            types.Decimal `json:"monthlyRevenue"`
	PreviousMonthlyRevenue    types.Decimal `json:"previousMonthlyRevenue"`
	MonthlyRevenueDeltaPct    float64       `json:"monthlyRevenueDeltaPct"`
	ActiveProductsCount       int64         `json:"activeProductsCount"`
	TotalProductsCount        int64         `json:"totalProductsCount"`
	TotalCustomersCount       int64         `json:"totalCustomersCount"`
	NewCustomersMonthCount    int64         `json:"newCustomersMonthCount"`
}

// RevenueSeriesPoint is one bucket of the revenue chart.
type RevenueSeriesPoint struct {
	Date         string        `json:"date"`
	Label        string        `json:"label"`
	Revenue      types.Decimal `json:"revenue"`
	SalesCount   int64         `json:"salesCount"`
	NewCustomers int64         `json:"newCustomers"`
}

// TopProductItem is a product ranked by sales in the selected period.
type TopProductItem struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Category   string        `json:"category"`
	SalesCount int64         `json:"salesCount"`
	Revenue    types.Decimal `json:"revenue"`
	Price      types.Decimal `json:"price"`
	Tone       *string       `json:"tone,omitempty"`
}

// AdminSale mirrors one row of the sales table.
type AdminSale struct {
	ID                string          `json:"id"`
	ExternalPaymentID string          `json:"externalPaymentId"`
	CustomerID        string          `json:"customerId"`
	CustomerName      string          `json:"customerName"`
	CustomerEmail     string          `json:"customerEmail"`
	ProductID         string          `json:"productId"`
	ProductTitle      string          `json:"productTitle"`
	ProductCategory   string          `json:"productCategory"`
	Amount            types.Decimal   `json:"amount"`
	PaymentMethod     string          `json:"paymentMethod"`
	Status            string          `json:"status"`
	StatusDetail      *string         `json:"statusDetail,omitempty"`
	AccessGranted     bool            `json:"accessGranted"`
	CreatedAt         types.Timestamp `json:"createdAt"`
}

// AdminActivity is one item of the dashboard activity feed.
type AdminActivity struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	CreatedAt    types.Timestamp `json:"createdAt"`
	RelativeTime string          `json:"relativeTime"`
}

// AdminCustomerPurchase is a nested purchase of an admin customer.
type AdminCustomerPurchase struct {
	SaleID          string          `json:"saleId"`
	ProductID       string          `json:"productId"`
	ProductTitle    string          `json:"productTitle"`
	ProductCategory string          `json:"productCategory"`
	Amount          types.Decimal   `json:"amount"`
	Status          string          `json:"status"`
	PaymentMethod   string          `json:"paymentMethod"`
	PurchasedAt     types.Timestamp `json:"purchasedAt"`
}

// AdminCustomer mirrors a customer row and its purchases.
type AdminCustomer struct {
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	Email          string                  `json:"email"`
	CreatedAt      types.Timestamp         `json:"createdAt"`
	PurchasesCount int64                   `json:"purchasesCount"`
	TotalSpent     types.Decimal           `json:"totalSpent"`
	LastPurchaseAt *types.Timestamp        `json:"lastPurchaseAt"`
	Purchases      []AdminCustomerPurchase `json:"purchases"`
}

// PeriodSummaryMetrics mirrors the report summary cards.
type PeriodSummaryMetrics struct {
	TotalRevenue          types.Decimal `json:"totalRevenue"`
	RevenueDeltaPct       float64       `json:"revenueDeltaPct"`
	ComparisonLabel       string        `json:"comparisonLabel"`
	AverageTicket         types.Decimal `json:"averageTicket"`
	AverageTicketDeltaPct float64       `json:"averageTicketDeltaPct"`
	ApprovalRate          float64       `json:"approvalRate"`
	ApprovalRateDeltaPp   float64       `json:"approvalRateDeltaPp"`
	RefundsCount          int64         `json:"refundsCount"`
	RefundsDelta          int64         `json:"refundsDelta"`
	TotalSalesCount       int64         `json:"totalSalesCount"`
	NewCustomersCount     int64         `json:"newCustomersCount"`
}

// WebhookStatus is the Mercado Pago webhook connectivity summary.
type WebhookStatus struct {
	Connected         bool   `json:"connected"`
	LastEventRelative string `json:"lastEventRelative"`
}

// AdminDashboardDTO is the body of GET /admin/dashboard. The frontend injects
// isDemo and productsLive; they are deliberately not part of this contract.
type AdminDashboardDTO struct {
	KPIs              DashboardKPIs        `json:"kpis"`
	PeriodSummary     PeriodSummaryMetrics `json:"periodSummary"`
	RevenueSeries     []RevenueSeriesPoint `json:"revenueSeries"`
	TopProducts       []TopProductItem     `json:"topProducts"`
	RecentSales       []AdminSale          `json:"recentSales"`
	RecentActivity    []AdminActivity      `json:"recentActivity"`
	PendingSalesCount int64                `json:"pendingSalesCount"`
	WebhookStatus     WebhookStatus        `json:"webhookStatus"`
}

// AdminSettings is the editable admin configuration document.
type AdminSettings struct {
	StoreName             string `json:"storeName"`
	SupportEmail          string `json:"supportEmail"`
	StorefrontEnabled     bool   `json:"storefrontEnabled"`
	AutoReleaseOnApproval bool   `json:"autoReleaseOnApproval"`
	NotifyOnNewSale       bool   `json:"notifyOnNewSale"`
	NotifyOnPendingPix    bool   `json:"notifyOnPendingPix"`
	MercadoPagoMode       string `json:"mercadoPagoMode"`
	WebhookURL            string `json:"webhookUrl"`
}

// UpdateAdminSettingsRequest is the body of PUT /admin/settings. Unknown
// fields are rejected by utils.BindJSON before Validate runs.
type UpdateAdminSettingsRequest struct {
	StoreName             string `json:"storeName" validate:"required"`
	SupportEmail          string `json:"supportEmail" validate:"required,email"`
	StorefrontEnabled     bool   `json:"storefrontEnabled"`
	AutoReleaseOnApproval bool   `json:"autoReleaseOnApproval"`
	NotifyOnNewSale       bool   `json:"notifyOnNewSale"`
	NotifyOnPendingPix    bool   `json:"notifyOnPendingPix"`
	MercadoPagoMode       string `json:"mercadoPagoMode" validate:"required,oneof=production sandbox"`
	WebhookURL            string `json:"webhookUrl" validate:"omitempty,url"`
}

var updateAdminSettingsMessages = utils.Messages{
	"storeName.required":       "storeName should not be empty",
	"supportEmail.required":    "supportEmail should not be empty",
	"supportEmail.email":       "supportEmail must be an email",
	"mercadoPagoMode.required": "mercadoPagoMode must be one of the following values: production, sandbox",
	"mercadoPagoMode.oneof":    "mercadoPagoMode must be one of the following values: production, sandbox",
	"webhookUrl.url":           "webhookUrl must be a URL address",
}

func (d UpdateAdminSettingsRequest) Validate() []string {
	return utils.Validate(d, updateAdminSettingsMessages)
}
