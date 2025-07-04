package paycell

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Mock Paycell API server
func createMockPaycellServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case endpointProvision:
			handleMockPayment(w, r, false)
		case endpointGetThreeDSession:
			handleMockPayment(w, r, true)
		case endpointInquire:
			// Extract paymentId from request body to determine status
			var reqData map[string]any
			json.NewDecoder(r.Body).Decode(&reqData)
			paymentID, _ := reqData["paymentId"].(string)

			if strings.Contains(paymentID, "success") {
				handleMockPaymentStatus(w, r, "SUCCESS")
			} else if strings.Contains(paymentID, "pending") {
				handleMockPaymentStatus(w, r, "PENDING")
			} else if strings.Contains(paymentID, "failed") {
				handleMockPaymentStatus(w, r, "FAILED")
			} else {
				handleMockPaymentStatus(w, r, "SUCCESS")
			}
		case endpointRefund:
			handleMockRefund(w, r)
		case endpointReverse:
			handleMockCancel(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "endpoint not found",
			})
		}
	}))
}

func handleMockPayment(w http.ResponseWriter, r *http.Request, is3D bool) {
	var reqData map[string]any
	json.NewDecoder(r.Body).Decode(&reqData)

	// Extract card number for test scenarios - now flat structure
	cardNumber, _ := reqData["cardNumber"].(string)
	amountStr, _ := reqData["amount"].(string)

	response := PaycellResponse{
		PaymentID:     "pay_" + generateMockID(),
		TransactionID: "txn_" + generateMockID(),
		Currency:      "TRY",
		Amount:        amountStr,
	}

	// Determine response based on test card numbers and amounts
	switch {
	case cardNumber == "5528790000000008": // Success card
		response.Success = true
		response.Status = statusSuccess
		response.Message = "Payment successful"

	case cardNumber == "5528790000000016": // Insufficient funds
		response.Success = false
		response.Status = statusFailed
		response.Message = "Insufficient funds"
		response.ErrorCode = errorCodeInsufficientFunds

	case cardNumber == "5528790000000024": // Invalid card
		response.Success = false
		response.Status = statusFailed
		response.Message = "Invalid card"
		response.ErrorCode = errorCodeInvalidCard

	case cardNumber == "5528790000000032": // Expired card
		response.Success = false
		response.Status = statusFailed
		response.Message = "Card expired"
		response.ErrorCode = errorCodeExpiredCard

	case cardNumber == "5528790000000040": // Declined card
		response.Success = false
		response.Status = statusFailed
		response.Message = "Card declined by bank"
		response.ErrorCode = errorCodeDeclined

	case cardNumber == "5528790000000057": // 3D redirect card (for 3D tests)
		if is3D {
			response.Success = false
			response.Status = statusPending
			response.Message = "3D authentication required"
			response.RedirectURL = "https://3ds.test.paycell.com/auth?token=mock_token"
			response.HTML = `<form action="https://3ds.test.paycell.com/auth" method="POST">
				<input type="hidden" name="token" value="mock_token">
				<input type="hidden" name="amount" value="` + amountStr + `">
			</form>`
		} else {
			response.Success = true
			response.Status = statusSuccess
			response.Message = "Payment successful"
		}

	case amountStr == "999.99": // Test amount that triggers timeout
		time.Sleep(35 * time.Second) // Simulate timeout
		response.Success = false
		response.Status = statusFailed
		response.Message = "Request timeout"
		response.ErrorCode = errorCodeSystemError

	default:
		response.Success = true
		response.Status = statusSuccess
		response.Message = "Payment successful"
	}

	w.Header().Set("Content-Type", "application/json")
	if !response.Success && response.Status == statusFailed {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

func handleMockPaymentStatus(w http.ResponseWriter, r *http.Request, status string) {
	response := PaycellResponse{
		Success:       status == "SUCCESS",
		Status:        status,
		PaymentID:     "pay_" + generateMockID(),
		TransactionID: "txn_" + generateMockID(),
		Amount:        "100.50",
		Currency:      "TRY",
	}

	switch status {
	case "SUCCESS":
		response.Message = "Payment completed successfully"
	case "PENDING":
		response.Message = "Payment is being processed"
	case "FAILED":
		response.Message = "Payment failed"
		response.ErrorCode = errorCodeDeclined
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleMockRefund(w http.ResponseWriter, r *http.Request) {
	var reqData map[string]any
	json.NewDecoder(r.Body).Decode(&reqData)

	refundAmount, _ := reqData["refundAmount"].(float64)

	response := PaycellResponse{
		Success:       true,
		Status:        statusRefunded,
		PaymentID:     reqData["paymentId"].(string),
		TransactionID: "ref_" + generateMockID(),
		Amount:        fmt.Sprintf("%.2f", refundAmount),
		Currency:      "TRY",
		Message:       "Refund processed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleMockCancel(w http.ResponseWriter, r *http.Request) {
	var reqData map[string]any
	json.NewDecoder(r.Body).Decode(&reqData)

	response := PaycellResponse{
		Success:       true,
		Status:        statusCancelled,
		PaymentID:     reqData["paymentId"].(string),
		TransactionID: "can_" + generateMockID(),
		Amount:        "100.50",
		Currency:      "TRY",
		Message:       "Payment cancelled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateMockID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}

func setupTestProvider(serverURL string) *PaycellProvider {
	p := NewProvider().(*PaycellProvider)
	config := map[string]string{
		"username":     "sandbox-paycell-username",
		"password":     "sandbox-paycell-password",
		"merchantId":   "sandbox-paycell-merchant-id",
		"terminalId":   "sandbox-paycell-terminal-id",
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.gopay.com",
	}
	p.Initialize(config)
	p.baseURL = serverURL
	p.paymentManagementURL = serverURL // Use same server for tests
	return p
}

func TestPaycellProvider_Integration_CreatePayment_Success(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008", // Success card
			ExpireMonth: "12",
			ExpireYear:  "2030",
			CVV:         "123",
		},
	}

	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status %v, got %v", provider.StatusSuccessful, response.Status)
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %f, got %f", request.Amount, response.Amount)
	}

	if response.Currency != request.Currency {
		t.Errorf("Expected currency %s, got %s", request.Currency, response.Currency)
	}

	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID")
	}

	if response.TransactionID == "" {
		t.Error("Expected non-empty transaction ID")
	}
}

func TestPaycellProvider_Integration_CreatePayment_InsufficientFunds(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000016", // Insufficient funds card
			ExpireMonth: "12",
			ExpireYear:  "2030",
			CVV:         "123",
		},
	}

	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if response.Success {
		t.Error("Expected failed payment")
	}

	if response.Status != provider.StatusFailed {
		t.Errorf("Expected status %v, got %v", provider.StatusFailed, response.Status)
	}

	if response.ErrorCode != errorCodeInsufficientFunds {
		t.Errorf("Expected error code %s, got %s", errorCodeInsufficientFunds, response.ErrorCode)
	}

	if !strings.Contains(response.Message, "Insufficient funds") {
		t.Errorf("Expected message to contain 'Insufficient funds', got %s", response.Message)
	}
}

func TestPaycellProvider_Integration_Create3DPayment_Success(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:      100.50,
		Currency:    "TRY",
		CallbackURL: "https://example.com/callback",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000057", // 3D redirect card
			ExpireMonth: "12",
			ExpireYear:  "2030",
			CVV:         "123",
		},
	}

	response, err := p.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("Create3DPayment failed: %v", err)
	}

	if response.Success {
		t.Error("Expected pending payment for 3D authentication")
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status %v, got %v", provider.StatusPending, response.Status)
	}

	if response.RedirectURL == "" {
		t.Error("Expected non-empty redirect URL for 3D authentication")
	}

	if !strings.Contains(response.RedirectURL, "3ds.test.paycell.com") {
		t.Errorf("Expected redirect URL to contain 3ds.test.paycell.com, got %s", response.RedirectURL)
	}

	if response.HTML == "" {
		t.Error("Expected non-empty HTML for 3D authentication form")
	}
}

func TestPaycellProvider_Integration_GetPaymentStatus(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	tests := []struct {
		name            string
		paymentID       string
		expectedStatus  provider.PaymentStatus
		expectedSuccess bool
	}{
		{
			name:            "successful payment status",
			paymentID:       "pay_success",
			expectedStatus:  provider.StatusSuccessful,
			expectedSuccess: true,
		},
		{
			name:            "pending payment status",
			paymentID:       "pay_pending",
			expectedStatus:  provider.StatusPending,
			expectedSuccess: false,
		},
		{
			name:            "failed payment status",
			paymentID:       "pay_failed",
			expectedStatus:  provider.StatusFailed,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := p.GetPaymentStatus(ctx, tt.paymentID)

			if err != nil {
				t.Fatalf("GetPaymentStatus failed: %v", err)
			}

			if response.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, response.Success)
			}

			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, response.Status)
			}
		})
	}
}

func TestPaycellProvider_Integration_RefundPayment(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	request := provider.RefundRequest{
		PaymentID:      "pay_success",
		RefundAmount:   50.25,
		Currency:       "TRY",
		Reason:         "Customer request",
		Description:    "Partial refund",
		ConversationID: "conv123",
	}

	response, err := p.RefundPayment(ctx, request)

	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful refund")
	}

	if response.RefundAmount != request.RefundAmount {
		t.Errorf("Expected refund amount %f, got %f", request.RefundAmount, response.RefundAmount)
	}

	if response.PaymentID != request.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", request.PaymentID, response.PaymentID)
	}

	if response.RefundID == "" {
		t.Error("Expected non-empty refund ID")
	}
}

func TestPaycellProvider_Integration_CancelPayment(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	response, err := p.CancelPayment(ctx, "pay_success", "Customer cancellation")

	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful cancellation")
	}

	if response.Status != provider.StatusCancelled {
		t.Errorf("Expected status %v, got %v", provider.StatusCancelled, response.Status)
	}

	if !strings.Contains(response.Message, "cancelled") {
		t.Errorf("Expected message to contain 'cancelled', got %s", response.Message)
	}
}

func TestPaycellProvider_Integration_ErrorScenarios(t *testing.T) {
	server := createMockPaycellServer()
	defer server.Close()

	p := setupTestProvider(server.URL)
	ctx := context.Background()

	tests := []struct {
		name         string
		cardNumber   string
		expectedCode string
		expectedMsg  string
	}{
		{
			name:         "invalid card",
			cardNumber:   "5528790000000024",
			expectedCode: errorCodeInvalidCard,
			expectedMsg:  "Invalid card",
		},
		{
			name:         "expired card",
			cardNumber:   "5528790000000032",
			expectedCode: errorCodeExpiredCard,
			expectedMsg:  "Card expired",
		},
		{
			name:         "declined card",
			cardNumber:   "5528790000000040",
			expectedCode: errorCodeDeclined,
			expectedMsg:  "Card declined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := provider.PaymentRequest{
				Amount:   100.50,
				Currency: "TRY",
				Customer: provider.Customer{
					Name:    "John",
					Surname: "Doe",
					Email:   "john@example.com",
				},
				CardInfo: provider.CardInfo{
					CardNumber:  tt.cardNumber,
					ExpireMonth: "12",
					ExpireYear:  "2030",
					CVV:         "123",
				},
			}

			response, err := p.CreatePayment(ctx, request)

			if err != nil {
				t.Fatalf("CreatePayment failed: %v", err)
			}

			if response.Success {
				t.Error("Expected failed payment")
			}

			if response.Status != provider.StatusFailed {
				t.Errorf("Expected status %v, got %v", provider.StatusFailed, response.Status)
			}

			if response.ErrorCode != tt.expectedCode {
				t.Errorf("Expected error code %s, got %s", tt.expectedCode, response.ErrorCode)
			}

			if !strings.Contains(response.Message, tt.expectedMsg) {
				t.Errorf("Expected message to contain '%s', got %s", tt.expectedMsg, response.Message)
			}
		})
	}
}

func TestPaycellProvider_Integration_ValidationErrors(t *testing.T) {
	p := &PaycellProvider{}

	ctx := context.Background()

	tests := []struct {
		name      string
		operation func() error
		expectErr string
	}{
		{
			name: "create payment with invalid request",
			operation: func() error {
				_, err := p.CreatePayment(ctx, provider.PaymentRequest{})
				return err
			},
			expectErr: "invalid payment request",
		},
		{
			name: "get payment status with empty ID",
			operation: func() error {
				_, err := p.GetPaymentStatus(ctx, "")
				return err
			},
			expectErr: "paymentID is required",
		},
		{
			name: "cancel payment with empty ID",
			operation: func() error {
				_, err := p.CancelPayment(ctx, "", "reason")
				return err
			},
			expectErr: "paymentID is required",
		},
		{
			name: "refund payment with empty ID",
			operation: func() error {
				_, err := p.RefundPayment(ctx, provider.RefundRequest{})
				return err
			},
			expectErr: "paymentID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()

			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if !strings.Contains(err.Error(), tt.expectErr) {
				t.Errorf("Expected error to contain '%s', got %s", tt.expectErr, err.Error())
			}
		})
	}
}
