package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type CustomerHandler struct {
	customerRepo      *repository.CustomerRepo
	paymentMethodRepo *repository.PaymentMethodRepo
}

func NewCustomerHandler(customerRepo *repository.CustomerRepo, paymentMethodRepo *repository.PaymentMethodRepo) *CustomerHandler {
	return &CustomerHandler{customerRepo: customerRepo, paymentMethodRepo: paymentMethodRepo}
}

// POST /merchants/:merchantId/customers
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	var req struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	customer := &model.Customer{
		ID:         uuid.New(),
		MerchantID: merchantID,
		Name:       req.Name,
		Email:      req.Email,
		Phone:      req.Phone,
	}
	if err := h.customerRepo.Create(c.Request.Context(), customer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create customer"})
		return
	}
	c.JSON(http.StatusCreated, customer)
}

// GET /merchants/:merchantId/customers
func (h *CustomerHandler) ListCustomers(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	customers, total, err := h.customerRepo.ListByMerchant(c.Request.Context(), merchantID, 1, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list customers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"customers": customers, "total": total})
}

// GET /merchants/:merchantId/customers/:customerId
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	customer, err := h.customerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}
	methods, _ := h.paymentMethodRepo.ListByCustomer(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"customer": customer, "payment_methods": methods})
}

// PUT /merchants/:merchantId/customers/:customerId
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("customerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	customer, err := h.customerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != "" {
		customer.Name = req.Name
	}
	if req.Email != "" {
		customer.Email = req.Email
	}
	if req.Phone != "" {
		customer.Phone = req.Phone
	}
	if err := h.customerRepo.Update(c.Request.Context(), customer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update customer"})
		return
	}
	c.JSON(http.StatusOK, customer)
}
