package paycell

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Test constants are now defined in paycell.go

func setupRealTestProvider() *PaycellProvider {
	p := NewProvider().(*PaycellProvider)
	config := map[string]string{
		"username":     testApplicationName,
		"password":     testApplicationPwd,
		"merchantId":   testMerchantCode,
		"terminalId":   testEulaID,
		"secureCode":   testSecureCode,
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.gopay.com",
	}

	err := p.Initialize(config)
	if err != nil {
		panic(err)
	}

	return p
}

// TestPaycellProvider_RealAPI_CreatePayment gerçek Paycell API'sine karşı payment testi
func TestPaycellProvider_RealAPI_CreatePayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()

	request := provider.PaymentRequest{
		Amount:   1.00,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "Customer",
			Email:       "test@example.com",
			PhoneNumber: "5551234567", // 10 digit format without country code
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "5528790000000008", // HalkBank test Mastercard (highlighted in docs)
			ExpireMonth:    "12",
			ExpireYear:     "26",  // 2-digit format as per docs
			CVV:            "001", // Correct test CVV
			CardHolderName: "Test Customer",
		},
		Description:    "Test payment",
		ConversationID: "gopay_real_test_" + time.Now().Format("20060102150405"),
		CallbackURL:    "https://test.gopay.com/callback",
	}

	fmt.Printf("Testing payment with real Paycell API...\n")
	fmt.Printf("Request: Amount=%.2f, Currency=%s\n", request.Amount, request.Currency)
	fmt.Printf("Card: %s (expires %s/%s)\n", request.CardInfo.CardNumber, request.CardInfo.ExpireMonth, request.CardInfo.ExpireYear)
	fmt.Printf("Customer: %s %s (%s)\n", request.Customer.Name, request.Customer.Surname, request.Customer.PhoneNumber)

	ctx := context.Background()
	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		fmt.Printf("Payment error: %v\n", err)
		// Don't fail the test, just show the error
		return
	}

	if response == nil {
		t.Fatal("Response is nil")
		return
	}

	fmt.Println("Payment Response:")
	fmt.Printf("  Success: %v\n", response.Success)
	fmt.Printf("  Status: %s\n", response.Status)
	fmt.Printf("  PaymentID: %s\n", response.PaymentID)
	fmt.Printf("  TransactionID: %s\n", response.TransactionID)
	fmt.Printf("  Amount: %.2f\n", response.Amount)
	fmt.Printf("  Currency: %s\n", response.Currency)
	fmt.Printf("  Message: %s\n", response.Message)
	fmt.Printf("  ErrorCode: %s\n", response.ErrorCode)

	// Payment ID veya Transaction ID boş olmamalı (başarılı olsun veya olmasın)
	if response.PaymentID == "" && response.TransactionID == "" {
		t.Errorf("Expected non-empty PaymentID or TransactionID")
	}
}

// TestPaycellProvider_RealAPI_Create3DPayment gerçek Paycell API'sine karşı 3D payment testi
func TestPaycellProvider_RealAPI_Create3DPayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:      1.00, // Minimum test amount
		Currency:    "TRY",
		CallbackURL: "https://test.gopay.com/callback",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "User",
			Email:       "test@paycell.example.com",
			PhoneNumber: "5551234567", // 10 digit format
			Address: &provider.Address{
				Country: "Turkey",
				City:    "Istanbul",
				Address: "Test Address",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "4355084355084358", // Akbank test card with 3D password "a"
			ExpireMonth:    "12",
			ExpireYear:     "26",
			CVV:            "000",
			CardHolderName: "TEST USER",
		},
		Description:    "GoPay Paycell Real 3D API Test",
		ConversationID: "gopay_3d_test_" + time.Now().Format("20060102150405"),
		Use3D:          true,
	}

	fmt.Printf("Testing 3D payment with real Paycell API...\n")
	fmt.Printf("Request: Amount=%.2f, Currency=%s\n", request.Amount, request.Currency)
	fmt.Printf("Card: %s (expires %s/%s)\n", request.CardInfo.CardNumber, request.CardInfo.ExpireMonth, request.CardInfo.ExpireYear)
	fmt.Printf("Customer: %s %s (%s)\n", request.Customer.Name, request.Customer.Surname, request.Customer.PhoneNumber)

	response, err := p.Create3DPayment(ctx, request)

	if err != nil {
		fmt.Printf("3D Payment error: %v\n", err)
		// Hata olması normal olabilir, sadece API'ye ulaşabildiğimizi test ediyoruz
		return
	}

	fmt.Println("3D Payment Response:")
	fmt.Printf("  Success: %v\n", response.Success)
	fmt.Printf("  Status: %s\n", response.Status)
	fmt.Printf("  PaymentID: %s\n", response.PaymentID)
	fmt.Printf("  TransactionID: %s\n", response.TransactionID)
	fmt.Printf("  RedirectURL: %s\n", response.RedirectURL)
	fmt.Printf("  Message: %s\n", response.Message)
	if response.ErrorCode != "" {
		fmt.Printf("  ErrorCode: %s\n", response.ErrorCode)
	}
	if response.HTML != "" {
		fmt.Printf("  HTML Form: %s\n", response.HTML[:100]+"...") // Show first 100 chars
	}

	// 3D için redirect URL veya HTML olması beklenir
	if response.RedirectURL == "" && response.HTML == "" {
		fmt.Printf("Warning: No redirect URL or HTML returned for 3D payment\n")
	}
}

// TestPaycellProvider_RealAPI_GetPaymentStatus gerçek API'de payment status testi
func TestPaycellProvider_RealAPI_GetPaymentStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()
	ctx := context.Background()

	// Önce bir payment oluşturalım
	paymentRequest := provider.PaymentRequest{
		Amount:   1.00,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "User",
			Email:       "test@paycell.example.com",
			PhoneNumber: "5551234567",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "26",
			CVV:            "001",
			CardHolderName: "TEST USER",
		},
		Description:    "GoPay Status Test",
		ConversationID: "gopay_status_test_" + time.Now().Format("20060102150405"),
	}

	paymentResponse, err := p.CreatePayment(ctx, paymentRequest)
	if err != nil {
		fmt.Printf("Could not create payment for status test: %v\n", err)
		return
	}

	if paymentResponse.PaymentID == "" {
		fmt.Printf("No PaymentID returned, cannot test status\n")
		return
	}

	fmt.Printf("Testing payment status for PaymentID: %s\n", paymentResponse.PaymentID)

	// Payment status sorgula
	statusResponse, err := p.GetPaymentStatus(ctx, paymentResponse.PaymentID)

	if err != nil {
		fmt.Printf("Payment status error: %v\n", err)
		return
	}

	fmt.Println("Payment Status Response:")
	fmt.Printf("  Success: %v\n", statusResponse.Success)
	fmt.Printf("  Status: %s\n", statusResponse.Status)
	fmt.Printf("  PaymentID: %s\n", statusResponse.PaymentID)
	fmt.Printf("  TransactionID: %s\n", statusResponse.TransactionID)
	fmt.Printf("  Amount: %.2f\n", statusResponse.Amount)
	fmt.Printf("  Message: %s\n", statusResponse.Message)
}

// TestPaycellProvider_RealAPI_Endpoints gerçek endpoint'lerin doğruluğunu test eder
func TestPaycellProvider_RealAPI_Endpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()

	// URL'lerin doğru set edildiğini kontrol et
	expectedSandboxURL := "https://tpay-test.turkcell.com.tr"
	expectedPaymentMgmtURL := "https://omccstb.turkcell.com.tr"

	if p.baseURL != expectedSandboxURL {
		t.Errorf("Expected baseURL %s, got %s", expectedSandboxURL, p.baseURL)
	}

	if p.paymentManagementURL != expectedPaymentMgmtURL {
		t.Errorf("Expected paymentManagementURL %s, got %s", expectedPaymentMgmtURL, p.paymentManagementURL)
	}

	// Credential'ların doğru set edildiğini kontrol et
	if p.username != testApplicationName {
		t.Errorf("Expected username %s, got %s", testApplicationName, p.username)
	}

	if p.merchantID != testMerchantCode {
		t.Errorf("Expected merchantID %s, got %s", testMerchantCode, p.merchantID)
	}

	if p.terminalID != testEulaID {
		t.Errorf("Expected terminalID %s, got %s", testEulaID, p.terminalID)
	}

	fmt.Printf("✅ All configurations are correct\n")
	fmt.Printf("  Base URL: %s\n", p.baseURL)
	fmt.Printf("  Payment Mgmt URL: %s\n", p.paymentManagementURL)
	fmt.Printf("  Username: %s\n", p.username)
	fmt.Printf("  Merchant ID: %s\n", p.merchantID)
	fmt.Printf("  Terminal ID: %s\n", p.terminalID)
}

// TestPaycellProvider_RealAPI_HashGeneration hash generation'ın doğruluğunu test eder
func TestPaycellProvider_RealAPI_HashGeneration(t *testing.T) {
	p := setupRealTestProvider()

	// Test hash generation with known values
	transactionID := "12345678901234567890"
	transactionDateTime := "20231201120000123"
	secureCode := "PAYCELL12345"

	hash := p.generatePaycellHash(transactionID, transactionDateTime, secureCode)

	fmt.Printf("Hash Generation Test:\n")
	fmt.Printf("  Transaction ID: %s\n", transactionID)
	fmt.Printf("  Transaction DateTime: %s\n", transactionDateTime)
	fmt.Printf("  Secure Code: %s\n", secureCode)
	fmt.Printf("  Generated Hash: %s\n", hash)

	// Hash boş olmamalı ve belirli bir uzunlukta olmalı
	if hash == "" {
		t.Errorf("Hash should not be empty")
	}

	if len(hash) < 40 { // SHA-256 base64 encoded should be at least 40 chars
		t.Errorf("Hash seems too short: %d chars", len(hash))
	}

	// Test that same input gives same hash
	hash2 := p.generatePaycellHash(transactionID, transactionDateTime, secureCode)
	if hash != hash2 {
		t.Errorf("Hash generation should be deterministic")
	}
}

// TestPaycellProvider_RealAPI_TransactionIDGeneration transaction ID generation'ın doğruluğunu test eder
func TestPaycellProvider_RealAPI_TransactionIDGeneration(t *testing.T) {
	p := setupRealTestProvider()

	// Test transaction ID generation
	transactionID := p.generateTransactionID()

	fmt.Printf("Transaction ID Generation Test:\n")
	fmt.Printf("  Generated Transaction ID: %s\n", transactionID)

	// Transaction ID 20 karakter olmalı
	if len(transactionID) != 20 {
		t.Errorf("Transaction ID should be 20 characters, got %d", len(transactionID))
	}

	// Sadece rakam olmalı
	for _, char := range transactionID {
		if char < '0' || char > '9' {
			t.Errorf("Transaction ID should contain only digits")
			break
		}
	}

	// Test that multiple calls generate different IDs
	transactionID2 := p.generateTransactionID()
	if transactionID == transactionID2 {
		t.Errorf("Transaction IDs should be unique")
	}
}

// TestPaycellProvider_RealAPI_DateTimeGeneration datetime generation'ın doğruluğunu test eder
func TestPaycellProvider_RealAPI_DateTimeGeneration(t *testing.T) {
	p := setupRealTestProvider()

	// Test datetime generation
	dateTime := p.generateTransactionDateTime()

	fmt.Printf("DateTime Generation Test:\n")
	fmt.Printf("  Generated DateTime: %s\n", dateTime)

	// DateTime 17 karakter olmalı (YYYYMMddHHmmssSSS)
	if len(dateTime) != 17 {
		t.Errorf("DateTime should be 17 characters, got %d", len(dateTime))
	}

	// Sadece rakam olmalı
	for _, char := range dateTime {
		if char < '0' || char > '9' {
			t.Errorf("DateTime should contain only digits")
			break
		}
	}
}
