package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/chrisprojs/Franchiso/config"
	"github.com/chrisprojs/Franchiso/models"
	"github.com/chrisprojs/Franchiso/utils"
)

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

type RegisterResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type GetProfileResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type VerifyEmailRequest struct {
	Email            string `json:"email" binding:"required,email"`
	VerificationCode string `json:"verification_code" binding:"required"`
}

type VerifyEmailResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type PendingUserData struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
	Code         string `json:"code"`
}

func Register(c *gin.Context, app *config.App) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email is already registered
	var existingUser models.User
	err := app.DB.Model(&existingUser).Where("email = ?", req.Email).Select()
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is already registered"})
		return
	}

	// Check if email already exists in Redis (pending verification)
	ctx := context.Background()
	pendingKey := fmt.Sprintf("pending_registration:%s", req.Email)
	exists, err := app.Redis.Exists(ctx, pendingKey).Result()
	if err == nil && exists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is in the verification process. Please check your email or wait a moment"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to hash password: %v", err)})
		return
	}

	// Generate verification code
	verificationCode, err := utils.GenerateVerificationCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification code"})
		return
	}

	// Save user data to Redis with 10 minute TTL
	userID := uuid.New()
	pendingUser := PendingUserData{
		ID:           userID.String(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         req.Role,
		Code:         verificationCode,
	}

	userDataJSON, err := json.Marshal(pendingUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare registration data"})
		return
	}

	// Save data with key: pending_registration:{email}
	err = app.Redis.Set(ctx, pendingKey, string(userDataJSON), 10*time.Minute).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store registration data"})
		return
	}

	// Initialize attempt counter to 0
	attemptKey := fmt.Sprintf("verification_attempt:%s", req.Email)
	err = app.Redis.Set(ctx, attemptKey, "0", 10*time.Minute).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize verification system"})
		return
	}

	// Send email with verification code
	err = SendVerificationEmail(app.Email, req.Email, req.Name, verificationCode)
	if err != nil {
		// Log the error but do not fail registration, because the data is already stored in Redis
		// The user can still try to verify using the generated code
		fmt.Printf("Warning: Failed to send verification email to %s: %v\n", req.Email, err)
		// Optionally delete data from Redis if you want to cancel the process when email fails to send
		// app.Redis.Del(ctx, pendingKey)
		// app.Redis.Del(ctx, attemptKey)
		// c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration succeeded but failed to send verification email. Please contact the administrator."})
		// return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Registration successful. Please check your email for the verification code",
	})
}

func VerifyEmail(c *gin.Context, app *config.App) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	pendingKey := fmt.Sprintf("pending_registration:%s", req.Email)
	attemptKey := fmt.Sprintf("verification_attempt:%s", req.Email)

	// Check whether registration data exists in Redis
	pendingData, err := app.Redis.Get(ctx, pendingKey).Result()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code is invalid or has expired. Please register again"})
		return
	}

	// Check attempt counter
	attemptStr, err := app.Redis.Get(ctx, attemptKey).Result()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read attempt data"})
		return
	}

	var attemptCount int
	fmt.Sscanf(attemptStr, "%d", &attemptCount)
	if attemptCount >= 3 {
		// Delete data from Redis because verification has failed 3 times
		app.Redis.Del(ctx, pendingKey)
		app.Redis.Del(ctx, attemptKey)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification attempts have reached the maximum limit (3 times). Please register again"})
		return
	}

	// Parse user data from Redis
	var pendingUser PendingUserData
	if err := json.Unmarshal([]byte(pendingData), &pendingUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca data registrasi"})
		return
	}

	// Verify code
	if pendingUser.Code != req.VerificationCode {
		// Increment attempt counter
		attemptCount++
		app.Redis.Set(ctx, attemptKey, fmt.Sprintf("%d", attemptCount), 10*time.Minute)
		remainingAttempts := 3 - attemptCount
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "Incorrect verification code",
			"remaining_attempts": remainingAttempts,
		})
		return
	}

	// Code is correct, save user to database
	userID, err := uuid.Parse(pendingUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user data"})
		return
	}

	user := models.User{
		ID:           userID,
		Name:         pendingUser.Name,
		Email:        pendingUser.Email,
		PasswordHash: pendingUser.PasswordHash,
		Role:         pendingUser.Role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = app.DB.Model(&user).Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save user: %v", err)})
		return
	}

	// Generate JWT token after verification is successful
	accessToken, err := utils.GenerateJWT(user.ID.String(), user.Role, "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate access token: %v", err)})
		return
	}
	refreshToken, err := utils.GenerateJWT(user.ID.String(), user.Role, "refresh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate refresh token: %v", err)})
		return
	}

	// Save refresh token to sessions table
	session := models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err = app.DB.Model(&session).Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Delete data from Redis because verification has succeeded
	app.Redis.Del(ctx, pendingKey)
	app.Redis.Del(ctx, attemptKey)

	resp := VerifyEmailResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserResponse{
			ID:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role,
		},
	}
	c.JSON(http.StatusOK, resp)
}

func Login(c *gin.Context, app *config.App) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := app.DB.Model(&user).Where("email = ?", req.Email).Select()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password is incorrect"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password is incorrect"})
		return
	}

	// Generate JWT token after user successfully logged in
	accessToken, err := utils.GenerateJWT(user.ID.String(), user.Role, "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate access token: %v", err)})
		return
	}
	refreshToken, err := utils.GenerateJWT(user.ID.String(), user.Role, "refresh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate refresh token: %v", err)})
		return
	}

	// Save refresh token to sessions table
	session := models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err = app.DB.Model(&session).Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserResponse{
			ID:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role,
		},
	}

	c.JSON(http.StatusOK, resp)
}

func GetProfile(c *gin.Context, app *config.App) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not authenticated"})
		return
	}

	var user models.User
	err := app.DB.Model(&user).Where("id = ?", userID).Select()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	resp := GetProfileResponse{
		ID:    user.ID.String(),
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
	}

	c.JSON(http.StatusOK, resp)
}
