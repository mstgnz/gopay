package ozanpay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// Test cards for OzanPay (these are fictional but follow standard test patterns)
	testCardSuccess      = "4111111111111111"
	testCardInsufficient = "4000000000000002"
	testCardDeclined     = "4000000000000341"
	testCard3DRedirect   = "4000000000000044"
	testCardExpired      = "4000000000000069"
	testCardInvalid      = "4000000000000127"
	testCardFraudulent   = "4000000000000259"

	// Test amounts that trigger specific responses
	testAmountSuccess      = 100.50
	testAmountInsufficient = 1.00
	testAmountDeclined     = 2.00
	testAmountFraudulent   = 666.00
)

func getTestConfig() map[string]string {
	return map[string]string{
		"apiKey":       config.GetEnv("OZANPAY_API_KEY", "test_api_key_ozanpay"),
		"secretKey":    config.GetEnv("OZANPAY_SECRET_KEY", "test_secret_key_ozanpay"),
		"merchantId":   config.GetEnv("OZANPAY_MERCHANT_ID", "test_merchant_id"),
		"environment":  config.GetEnv("OZANPAY_ENVIRONMENT", "sandbox"),
		"gopayBaseURL": "https://test.gopay.com",
	}
}

func createMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var requestBody map[string]any
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&requestBody)
		}

		// Default response
		response := map[string]any{
			"id":            "ozp_" + generateTestID(),
			"transactionId": "txn_" + generateTestID(),
			"currency":      "USD",
			"systemTime":    time.Now().Format(time.RFC3339),
		}

		// Handle different endpoints
		switch {
		case strings.Contains(r.URL.Path, "/payments") && r.Method == "POST":
			handlePaymentRequest(requestBody, response)
		case strings.Contains(r.URL.Path, "/payments") && r.Method == "GET":
			handlePaymentStatus(r.URL.Path, response)
		case strings.Contains(r.URL.Path, "/refunds") && r.Method == "POST":
			handleRefundRequest(requestBody, response)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func handlePaymentRequest(requestBody map[string]any, response map[string]any) {
	card, _ := requestBody["card"].(map[string]any)
	cardNumber, _ := card["number"].(string)
	amount, _ := requestBody["amount"].(float64)

	response["amount"] = amount

	// Simulate different card responses
	switch cardNumber {
	case testCardSuccess:
		response["status"] = statusApproved
	case testCardInsufficient:
		response["status"] = statusFailed
		response["errorCode"] = errorCodeInsufficientFunds
		response["errorMessage"] = "Insufficient funds"
	case testCardDeclined:
		response["status"] = statusDeclined
		response["errorCode"] = errorCodeDeclined
		response["errorMessage"] = "Card declined"
	case testCard3DRedirect:
		response["status"] = statusPending
		response["redirectUrl"] = "https://3ds.ozanpay.com/redirect?token=test_token"
	case testCardExpired:
		response["status"] = statusFailed
		response["errorCode"] = errorCodeExpiredCard
		response["errorMessage"] = "Card expired"
	case testCardInvalid:
		response["status"] = statusFailed
		response["errorCode"] = errorCodeInvalidCard
		response["errorMessage"] = "Invalid card number"
	case testCardFraudulent:
		response["status"] = statusFailed
		response["errorCode"] = errorCodeFraudulent
		response["errorMessage"] = "Fraudulent transaction detected"
	default:
		// Check amount-based responses
		switch amount {
		case float64(testAmountInsufficient * 100):
			response["status"] = statusFailed
			response["errorCode"] = errorCodeInsufficientFunds
			response["errorMessage"] = "Insufficient funds"
		case float64(testAmountDeclined * 100):
			response["status"] = statusDeclined
			response["errorCode"] = errorCodeDeclined
			response["errorMessage"] = "Payment declined"
		case float64(testAmountFraudulent * 100):
			response["status"] = statusFailed
			response["errorCode"] = errorCodeFraudulent
			response["errorMessage"] = "Fraudulent transaction"
		default:
			response["status"] = statusApproved
		}
	}
}

func handlePaymentStatus(path string, response map[string]any) {
	// Extract payment ID from path
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		paymentID := parts[len(parts)-1]
		response["id"] = paymentID

		// Simulate different statuses based on payment ID pattern
		if strings.Contains(paymentID, "failed") {
			response["status"] = statusFailed
			response["errorCode"] = errorCodeDeclined
			response["errorMessage"] = "Payment failed"
		} else if strings.Contains(paymentID, "pending") {
			response["status"] = statusPending
		} else {
			response["status"] = statusApproved
			response["amount"] = float64(10050) // Default test amount
		}
	}
}

func handleRefundRequest(requestBody map[string]any, response map[string]any) {
	parentID, _ := requestBody["parentId"].(string)
	amount, _ := requestBody["amount"].(float64)

	response["id"] = "ref_" + generateTestID()
	response["parentId"] = parentID
	response["amount"] = amount

	// Simulate refund based on parent payment
	if strings.Contains(parentID, "failed") {
		response["status"] = statusFailed
		response["errorCode"] = "REFUND_FAILED"
		response["errorMessage"] = "Cannot refund failed payment"
	} else {
		response["status"] = statusApproved
	}
}

func generateTestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}

func TestOzanPayProvider_Integration_CreatePayment_Success(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: "USD",
		Customer: provider.Customer{
			ID:      "customer_123",
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: provider.Address{
				Country: "US",
				City:    "New York",
				Address: "123 Test Street",
				ZipCode: "10001",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess,
			CVV:            "123",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
		},
		Description: "Test payment",
	}

	ctx := context.Background()
	response, err := ozanpayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status successful, got %v", response.Status)
	}

	if response.Amount != testAmountSuccess {
		t.Errorf("Expected amount %.2f, got %.2f", testAmountSuccess, response.Amount)
	}

	if response.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", response.Currency)
	}

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}
}

func TestOzanPayProvider_Integration_CreatePayment_InsufficientFunds(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	request := provider.PaymentRequest{
		Amount:   testAmountInsufficient,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  testCardInsufficient,
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()
	response, err := ozanpayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if response.Success {
		t.Error("Expected failed payment")
	}

	if response.Status != provider.StatusFailed {
		t.Errorf("Expected status failed, got %v", response.Status)
	}

	if response.ErrorCode != errorCodeInsufficientFunds {
		t.Errorf("Expected error code %s, got %s", errorCodeInsufficientFunds, response.ErrorCode)
	}
}

func TestOzanPayProvider_Integration_Create3DPayment_Success(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  testCard3DRedirect,
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
		CallbackURL: "https://example.com/callback",
		Use3D:       true,
	}

	ctx := context.Background()
	response, err := ozanpayProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("Create3DPayment failed: %v", err)
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status pending, got %v", response.Status)
	}

	if response.RedirectURL == "" {
		t.Error("Expected redirect URL for 3D payment")
	}

	if !strings.Contains(response.RedirectURL, "3ds.ozanpay.com") {
		t.Errorf("Expected 3D redirect URL, got %s", response.RedirectURL)
	}
}

func TestOzanPayProvider_Integration_Complete3DPayment(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	ctx := context.Background()
	paymentID := "ozp_123456"
	conversationID := "conv_123"
	callbackData := map[string]string{
		"status": "approved",
		"token":  "3d_token_123",
	}

	response, err := ozanpayProvider.Complete3DPayment(ctx, paymentID, conversationID, callbackData)

	if err != nil {
		t.Fatalf("Complete3DPayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful 3D completion")
	}

	if response.PaymentID != paymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentID, response.PaymentID)
	}
}

func TestOzanPayProvider_Integration_GetPaymentStatus(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	ctx := context.Background()

	tests := []struct {
		name            string
		paymentID       string
		expectedStatus  provider.PaymentStatus
		expectedSuccess bool
	}{
		{
			name:            "Successful payment status",
			paymentID:       "ozp_success_123",
			expectedStatus:  provider.StatusSuccessful,
			expectedSuccess: true,
		},
		{
			name:            "Failed payment status",
			paymentID:       "ozp_failed_456",
			expectedStatus:  provider.StatusFailed,
			expectedSuccess: false,
		},
		{
			name:            "Pending payment status",
			paymentID:       "ozp_pending_789",
			expectedStatus:  provider.StatusPending,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := ozanpayProvider.GetPaymentStatus(ctx, tt.paymentID)

			if err != nil {
				t.Fatalf("GetPaymentStatus failed: %v", err)
			}

			if response.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, response.Success)
			}

			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, response.Status)
			}

			if response.PaymentID != tt.paymentID {
				t.Errorf("Expected payment ID %s, got %s", tt.paymentID, response.PaymentID)
			}
		})
	}
}

func TestOzanPayProvider_Integration_RefundPayment_Success(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	refundRequest := provider.RefundRequest{
		PaymentID:      "ozp_success_123",
		RefundAmount:   50.0,
		Currency:       "USD",
		Reason:         "Customer request",
		Description:    "Test refund",
		ConversationID: "conv_refund_123",
	}

	ctx := context.Background()
	response, err := ozanpayProvider.RefundPayment(ctx, refundRequest)

	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful refund")
	}

	if response.RefundAmount != 50.0 {
		t.Errorf("Expected refund amount 50.0, got %.2f", response.RefundAmount)
	}

	if response.PaymentID != refundRequest.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", refundRequest.PaymentID, response.PaymentID)
	}

	if response.RefundID == "" {
		t.Error("RefundID should not be empty")
	}
}

func TestOzanPayProvider_Integration_CancelPayment(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL

	ctx := context.Background()
	paymentID := "ozp_success_123"
	reason := "Customer requested cancellation"

	response, err := ozanpayProvider.CancelPayment(ctx, paymentID, reason)

	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful cancellation")
	}

	if response.Status != provider.StatusCancelled {
		t.Errorf("Expected status cancelled, got %v", response.Status)
	}

	if response.PaymentID != paymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentID, response.PaymentID)
	}
}

func TestOzanPayProvider_Integration_ValidateWebhook(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)

	webhookData := map[string]string{
		"id":       "ozp_webhook_123",
		"status":   "APPROVED",
		"amount":   "10050",
		"currency": "USD",
	}

	// Calculate correct signature
	rawJson, _ := json.Marshal(webhookData)
	correctSignature := ozanpayProvider.generateSignature(string(rawJson))

	headers := map[string]string{
		"X-Ozan-Signature": correctSignature,
	}

	ctx := context.Background()
	valid, result, err := ozanpayProvider.ValidateWebhook(ctx, webhookData, headers)

	if err != nil {
		t.Fatalf("ValidateWebhook failed: %v", err)
	}

	if !valid {
		t.Error("Expected valid webhook")
	}

	if result["paymentId"] != "ozp_webhook_123" {
		t.Errorf("Expected payment ID ozp_webhook_123, got %v", result["paymentId"])
	}

	if result["status"] != "APPROVED" {
		t.Errorf("Expected status APPROVED, got %v", result["status"])
	}
}

func TestOzanPayProvider_Integration_ErrorScenarios(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)

	ctx := context.Background()

	// Test invalid payment request
	invalidRequest := provider.PaymentRequest{
		Amount: 0, // Invalid amount
	}

	_, err := ozanpayProvider.CreatePayment(ctx, invalidRequest)
	if err == nil {
		t.Error("Expected error for invalid payment request")
	}

	// Test missing payment ID for status check
	_, err = ozanpayProvider.GetPaymentStatus(ctx, "")
	if err == nil {
		t.Error("Expected error for empty payment ID")
	}

	// Test missing payment ID for refund
	invalidRefund := provider.RefundRequest{
		PaymentID: "", // Empty payment ID
	}

	_, err = ozanpayProvider.RefundPayment(ctx, invalidRefund)
	if err == nil {
		t.Error("Expected error for empty payment ID in refund")
	}
}

func TestOzanPayProvider_Integration_NetworkTimeout(t *testing.T) {
	// Create a server that never responds to simulate timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Simulate slow response
	}))
	defer server.Close()

	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	ozanpayProvider.Initialize(config)
	ozanpayProvider.baseURL = server.URL
	ozanpayProvider.client.Timeout = 100 * time.Millisecond // Short timeout

	request := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  testCardSuccess,
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()
	_, err := ozanpayProvider.CreatePayment(ctx, request)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout-related error, got: %v", err)
	}
}
