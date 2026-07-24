package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type MerchantHandler struct {
	merchantRepo *repository.MerchantRepo
}

func NewMerchantHandler(merchantRepo *repository.MerchantRepo) *MerchantHandler {
	return &MerchantHandler{merchantRepo: merchantRepo}
}

// POST /merchants
func (h *MerchantHandler) CreateMerchant(c *gin.Context) {
	var req struct {
		LegalName string `json:"legal_name" binding:"required"`
		TradeName string `json:"trade_name"`
		Email     string `json:"email" binding:"required,email"`
		Phone     string `json:"phone"`
		Country   string `json:"country" binding:"required"`
		Currency  string `json:"currency" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	merchant := &model.Merchant{
		ID:        uuid.New(),
		LegalName: req.LegalName,
		TradeName: req.TradeName,
		Email:     req.Email,
		Phone:     req.Phone,
		Country:   req.Country,
		Currency:  req.Currency,
		Status:    model.MerchantStatusPending,
		KycStatus: model.KycStatusPending,
	}
	if err := h.merchantRepo.Create(c.Request.Context(), merchant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create merchant"})
		return
	}
	c.JSON(http.StatusCreated, merchant)
}

// GET /merchants
func (h *MerchantHandler) ListMerchants(c *gin.Context) {
	page, pageSize := 1, 20
	merchants, total, err := h.merchantRepo.List(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list merchants"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"merchants": merchants, "total": total})
}

// GET /merchants/:merchantId
func (h *MerchantHandler) GetMerchant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	merchant, err := h.merchantRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merchant not found"})
		return
	}
	c.JSON(http.StatusOK, merchant)
}

// PUT /merchants/:merchantId
func (h *MerchantHandler) UpdateMerchant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	merchant, err := h.merchantRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merchant not found"})
		return
	}
	var req struct {
		LegalName string `json:"legal_name"`
		TradeName string `json:"trade_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.LegalName != "" {
		merchant.LegalName = req.LegalName
	}
	if req.TradeName != "" {
		merchant.TradeName = req.TradeName
	}
	if req.Email != "" {
		merchant.Email = req.Email
	}
	if req.Phone != "" {
		merchant.Phone = req.Phone
	}
	if err := h.merchantRepo.Update(c.Request.Context(), merchant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update merchant"})
		return
	}
	c.JSON(http.StatusOK, merchant)
}
