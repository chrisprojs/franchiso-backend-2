package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/chrisprojs/Franchiso/config"
	"github.com/chrisprojs/Franchiso/models"
	"github.com/chrisprojs/Franchiso/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	midtrans "github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	snap "github.com/midtrans/midtrans-go/snap"
	"google.golang.org/genai"
)

// PaymentHandler interface to handle various types of payments
type PaymentHandler interface {
	// GetPaymentType returns the payment type (e.g., "boost", "subscription", etc.)
	GetPaymentType() string
	
	// ValidateOrderID validates whether the order_id is valid for this handler
	ValidateOrderID(orderID string, app *config.App) (bool, error)
	
	// ProcessPayment processes the payment after it has been successfully paid
	ProcessPayment(orderID string, notif *coreapi.TransactionStatusResponse, app *config.App) error
}

type BoostFranchiseRequest struct {
	Package string `json:"package" binding:"required"` // ex: "7", "14", "30"
}

type BoostFranchiseResponse struct {
	SnapToken   string `json:"snap_token"`
	RedirectUrl string `json:"redirect_url"`
}

func BoostFranchise(c *gin.Context, app *config.App) {
	franchiseID := c.Param("id")
	var req BoostFranchiseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := c.GetString("role")
	if role != "Franchisor" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User does not have access"})
		return
	}

	// Validasi franchise milik user
	// Hitung harga paket
	var price int
	switch req.Package {
	case "7":
		price = 100000
	case "14":
		price = 180000
	case "30":
		price = 350000
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid package"})
		return
	}

	// Create Boost data (is_active: false)
	packageDay, err := strconv.Atoi(req.Package)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s", err)})
		return
	}

	boost := models.Boost{
		ID:          uuid.New(),
		FranchiseID: uuid.MustParse(franchiseID),
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 0, packageDay),
		IsActive:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err = app.DB.Model(&boost).Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create boost"})
		return
	}

	// Buat Snap Token Midtrans
	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  boost.ID.String(),
			GrossAmt: int64(price),
		},
	}
	snapClient := app.Midtrans.SnapClient
	snapResp, err := snapClient.CreateTransaction(snapReq)
	if snapResp.Token == "" || snapResp.RedirectURL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create payment: %v", err)})
		return
	}

	res := BoostFranchiseResponse{
		SnapToken:   snapResp.Token,
		RedirectUrl: snapResp.RedirectURL,
	}

	c.JSON(http.StatusOK, res)
}

// BoostPaymentHandler implements PaymentHandler for boost payments
type BoostPaymentHandler struct{}

func (h *BoostPaymentHandler) GetPaymentType() string {
	return "boost"
}

func (h *BoostPaymentHandler) ValidateOrderID(orderID string, app *config.App) (bool, error) {
	boost := &models.Boost{}
	err := app.DB.Model(boost).Where("id = ?", orderID).Select()
	if err != nil {
		return false, nil // Order ID not found, not an error
	}
	return true, nil
}

func (h *BoostPaymentHandler) ProcessPayment(orderID string, notif *coreapi.TransactionStatusResponse, app *config.App) error {
	// Get boost by order_id
	boost := &models.Boost{}
	err := app.DB.Model(boost).Where("id = ?", orderID).Select()
	if err != nil {
		return fmt.Errorf("boost not found: %v", err)
	}
	
	if boost.IsActive {
		return fmt.Errorf("boost sudah aktif")
	}

	// If payment is successful, activate boost & franchise
	if notif.TransactionStatus == "settlement" || notif.TransactionStatus == "capture" {
		_, err = app.DB.Model((*models.Boost)(nil)).
			Set("is_active = ?", true).
			Set("updated_at = ?", time.Now()).
			Where("id = ?", boost.ID).
			Update()
		if err != nil {
			return fmt.Errorf("failed to update Boost: %v", err)
		}

		// Update franchise directly without select (avoid N+1)
		_, err = app.DB.Model((*models.Franchise)(nil)).
			Set("is_boosted = ?", true).
			Set("updated_at = ?", time.Now()).
			Where("id = ?", boost.FranchiseID).
			Update()
		if err != nil {
			return fmt.Errorf("failed to update Franchise in PostgreSQL: %v", err)
		}

		// Get franchise data for generating embedding
		franchise := &models.Franchise{}
		err = app.DB.Model(franchise).Where("id = ?", boost.FranchiseID).Select()
		if err != nil {
		return fmt.Errorf("failed to fetch franchise data: %v", err)
		}

		// Prepare update doc for Elasticsearch
		updateDoc := map[string]interface{}{
			"is_boosted": true,
			"updated_at": time.Now(),
		}

		// Generate text embedding if Gemini is active and franchise is boosted
		if os.Getenv("GEMINI_ACTIVE") == "true" && app.Gemini != nil {
			textForEmbedding := franchise.Brand + " " + franchise.Description
			embeddingRes, err := app.Gemini.Models.EmbedContent(context.Background(), "text-embedding-004", genai.Text(textForEmbedding), nil)
			if err != nil {
				return fmt.Errorf("failed to generate embedding: %v", err)
			}
			if len(embeddingRes.Embeddings) > 0 {
				// Convert []float32 to []float64 for Elasticsearch
				textVector := make([]float64, len(embeddingRes.Embeddings[0].Values))
				for i, v := range embeddingRes.Embeddings[0].Values {
					textVector[i] = float64(v)
				}
				updateDoc["text_vector"] = textVector
			}
		}

		// Synchronize to Elasticsearch
		_, err = app.ES.Update().
			Index("franchises").
			Id(boost.FranchiseID.String()).
			Doc(updateDoc).
			Do(context.Background())
		if err != nil {
			return fmt.Errorf("failed to update Franchise in Elasticsearch: %v", err)
		}
	}

	return nil
}

// PaymentCallback is a generalized function to handle all types of payment callbacks from Midtrans
func PaymentCallback(c *gin.Context, app *config.App) {
	var notif coreapi.TransactionStatusResponse
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validasi status transaksi
	if notif.TransactionStatus == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaction status must not be empty"})
		return
	}

	// Skip if status is not settlement or capture (pending, cancel, etc.)
	if notif.TransactionStatus != "settlement" && notif.TransactionStatus != "capture" {
		c.JSON(http.StatusContinue, gin.H{"message": fmt.Sprintf("Status transaksi: %s", notif.TransactionStatus)})
		return
	}

	// Find the appropriate handler based on order_id
	handler := findPaymentHandler(notif.OrderID, app)
	if handler == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("No handler found for order_id: %s", notif.OrderID)})
		return
	}

	// Record payment to the database
	transactionTime, err := utils.ParseStringToTime(notif.TransactionTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse transaction time: %v", err)})
		return
	}
	
	grossAmount, err := strconv.ParseFloat(notif.GrossAmount, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse payment amount: %v", err)})
		return
	}

	// Parse order_id as UUID for BoostID (can be changed if needed)
	orderUUID, err := uuid.Parse(notif.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid order ID: %v", err)})
		return
	}

	payment := models.Payment{
		ID:                uuid.New(),
		BoostID:           orderUUID,
		TransactionID:     uuid.MustParse(notif.TransactionID),
		GrossAmount:       float64(grossAmount),
		PaymentType:       notif.PaymentType,
		TransactionTime:   transactionTime,
		TransactionStatus: notif.TransactionStatus,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	
	_, err = app.DB.Model(&payment).Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to record payment: %s", err)})
		return
	}

	// Process payment according to the handler
	if err := handler.ProcessPayment(notif.OrderID, &notif, app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process payment: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

// findPaymentHandler finds the appropriate handler based on order_id
// The handler is determined by validating the order_id against each registered handler
func findPaymentHandler(orderID string, app *config.App) PaymentHandler {
	// List all available handlers
	handlers := []PaymentHandler{
		&BoostPaymentHandler{},
		// Add other handlers here if needed (e.g., SubscriptionPaymentHandler, etc.)
	}

	// Find handler that can validate the order_id
	for _, handler := range handlers {
		valid, err := handler.ValidateOrderID(orderID, app)
		if err != nil {
			continue // Skip if there is an error
		}
		if valid {
			return handler
		}
	}

	return nil
}

func BoostPurchaseCallback(c *gin.Context, app *config.App) {
	PaymentCallback(c, app)
}
