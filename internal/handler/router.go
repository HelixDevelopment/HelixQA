package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/websocket"
)

func NewRouter(logger *zap.Logger, authHandler *AuthHandler, userHandler *UserHandler, apiKeyHandler *ApiKeyHandler, merchantHandler *MerchantHandler, paymentHandler *PaymentHandler, customerHandler *CustomerHandler, subscriptionHandler *SubscriptionHandler, invoiceHandler *InvoiceHandler, payoutHandler *PayoutHandler, disputeHandler *DisputeHandler, webhookHandler *WebhookHandler, analyticsHandler *AnalyticsHandler, providerHandler *ProviderHandler, paymentMethodHandler *PaymentMethodHandler, exchangeRateHandler *ExchangeRateHandler, auditHandler *AuditHandler, webhookIngressHandler *WebhookIngressHandler, billingHandler *BillingHandler, healthHandler *HealthHandler, wsHandler *websocket.WSHandler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", healthHandler.Health)
	router.GET("/health/ready", healthHandler.Readiness)
	router.GET("/health/live", healthHandler.Liveness)
	router.GET("/metrics", MetricsHandler())
	router.GET("/ws", wsHandler.HandleWebSocket)

	v1 := router.Group("/api/v1")
	{
		// Auth (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/logout", authHandler.Logout)
			auth.POST("/mfa/setup", authHandler.SetupMFA)
			auth.POST("/mfa/verify", authHandler.VerifyMFA)
		}

		// Protected
		p := v1.Group("")
		{
			// Users
			p.GET("/users/me", userHandler.GetUser)
			p.PUT("/users/me", userHandler.UpdateUser)

			// API Keys
			p.POST("/api-keys", apiKeyHandler.CreateApiKey)
			p.GET("/api-keys", apiKeyHandler.ListApiKeys)
			p.DELETE("/api-keys/:keyId", apiKeyHandler.RevokeApiKey)

			// Merchants
			m := p.Group("/merchants")
			{
				m.GET("", merchantHandler.ListMerchants)
				m.POST("", merchantHandler.CreateMerchant)
				m.GET("/:merchantId", merchantHandler.GetMerchant)
				m.PUT("/:merchantId", merchantHandler.UpdateMerchant)

				// Customers
				c := m.Group("/:merchantId/customers")
				{
					c.GET("", customerHandler.ListCustomers)
					c.POST("", customerHandler.CreateCustomer)
					c.GET("/:customerId", customerHandler.GetCustomer)
					c.PUT("/:customerId", customerHandler.UpdateCustomer)
				}

				// Transactions
				t := m.Group("/:merchantId/transactions")
				{
					t.GET("", paymentHandler.ListTransactions)
					t.POST("", paymentHandler.ProcessPayment)
					t.GET("/:transactionId", paymentHandler.GetTransaction)
				}

				// Refunds
				r := m.Group("/:merchantId/refunds")
				{
					r.POST("", paymentHandler.CreateRefund)
				}

				// Subscriptions
				s := m.Group("/:merchantId/subscriptions")
				{
					s.GET("", subscriptionHandler.ListSubscriptions)
					s.POST("", subscriptionHandler.CreateSubscription)
					s.GET("/:subscriptionId", subscriptionHandler.GetSubscription)
					s.PATCH("/:subscriptionId", subscriptionHandler.UpdateSubscription)
					s.DELETE("/:subscriptionId", subscriptionHandler.CancelSubscription)
				}

				// Invoices
				inv := m.Group("/:merchantId/invoices")
				{
					inv.GET("", invoiceHandler.ListInvoices)
					inv.POST("", invoiceHandler.CreateInvoice)
					inv.GET("/:invoiceId", invoiceHandler.GetInvoice)
				}

			// Payouts
			pay := m.Group("/:merchantId/payouts")
				{
					pay.GET("", payoutHandler.ListPayouts)
					pay.GET("/:payoutId", payoutHandler.GetPayout)
					pay.POST("", payoutHandler.CreatePayout)
				}

				// Disputes
				d := m.Group("/:merchantId/disputes")
				{
					d.GET("", disputeHandler.ListDisputes)
					d.GET("/:disputeId", disputeHandler.GetDispute)
					d.POST("", disputeHandler.CreateDispute)
					d.POST("/:disputeId/evidence", disputeHandler.AddEvidence)
				}

			// Payment Methods
			pm := m.Group("/:merchantId/payment-methods")
				{
					pm.GET("", paymentMethodHandler.ListPaymentMethods)
					pm.POST("", paymentMethodHandler.CreatePaymentMethod)
					pm.GET("/:paymentMethodId", paymentMethodHandler.GetPaymentMethod)
					pm.DELETE("/:paymentMethodId", paymentMethodHandler.DeletePaymentMethod)
				}

			// Webhooks
			wh := m.Group("/:merchantId/webhooks")
				{
					wh.GET("", webhookHandler.ListWebhooks)
					wh.POST("", webhookHandler.CreateWebhook)
					wh.GET("/:webhookId", webhookHandler.GetWebhook)
					wh.PUT("/:webhookId", webhookHandler.UpdateWebhook)
					wh.DELETE("/:webhookId", webhookHandler.DeleteWebhook)
				}

			// Provider Configs
			pr := m.Group("/:merchantId/providers")
				{
					pr.GET("", providerHandler.ListProviders)
					pr.POST("", providerHandler.CreateProvider)
					pr.GET("/:providerId", providerHandler.GetProvider)
					pr.PUT("/:providerId", providerHandler.UpdateProvider)
					pr.DELETE("/:providerId", providerHandler.DeleteProvider)
				}

			// Exchange Rates
			m.GET("/:merchantId/exchange-rates", exchangeRateHandler.GetExchangeRate)

				// Analytics
				a := m.Group("/:merchantId/analytics")
				{
					a.GET("/summary", analyticsHandler.GetSummary)
					a.GET("/transactions", analyticsHandler.GetTransactionAnalytics)
					a.GET("/export", analyticsHandler.ExportTransactions)
				}

			// Audit Logs
			audit := m.Group("/:merchantId/audit-logs")
				{
					audit.GET("", auditHandler.ListAuditLogs)
				}

			// Billing
			b := m.Group("/:merchantId/billing")
				{
					b.GET("/fees", billingHandler.GetFees)
					b.GET("/invoices", billingHandler.GetBillingInvoices)
				}
			}
		}

		// Webhook ingress (no auth)
		wh := v1.Group("/webhooks")
		{
			wh.POST("/stripe", webhookIngressHandler.HandleStripe)
			wh.POST("/paypal", webhookIngressHandler.HandlePayPal)
			wh.POST("/square", webhookIngressHandler.HandleSquare)
		}
	}

	return router
}
