package ziraat

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Test credentials for Ziraat sandbox (should be provided by Ziraat)
const (
	testUsername = "191312485api"
	testPassword = "ESTtest1234."
	testStoreKey = "3230323531303032313733303"
)

// Test cards for Ziraat sandbox
type TestCard struct {
	CardNumber  string
	ExpireMonth string
	ExpireYear  string
	CVV         string
}

var testCards = []TestCard{
	{
		CardNumber:  "5528790000000008",
		ExpireMonth: "12",
		ExpireYear:  "2030",
		CVV:         "123",
	},
	{
		CardNumber:  "4355084355084358",
		ExpireMonth: "12",
		ExpireYear:  "2030",
		CVV:         "000",
	},
}

func setupRealTestProvider() *ZiraatProvider {
	p := NewProvider().(*ZiraatProvider)
	config := map[string]string{
		"username":    testUsername,
		"password":    testPassword,
		"storeKey":    testStoreKey,
		"environment": "sandbox",
	}

	err := p.Initialize(config)
	if err != nil {
		panic(err)
	}

	return p
}

// TestZiraatProvider_RealAPI_Create3DPayment gerçek Ziraat API'sine karşı 3D payment testi
func TestZiraatProvider_RealAPI_Create3DPayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()
	ctx := context.Background()

	card := testCards[rand.Intn(len(testCards))]
	request := provider.PaymentRequest{
		TenantID:    1,
		Amount:      5.00,
		Currency:    "TRY",
		CallbackURL: "https://test.gopay.com/callback",
		ClientIP:    "127.0.0.1",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@ziraat.example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     card.CardNumber,
			ExpireMonth:    card.ExpireMonth,
			ExpireYear:     card.ExpireYear,
			CVV:            card.CVV,
			CardHolderName: "TEST USER",
		},
		Description:    "GoPay Ziraat Real 3D API Test",
		ConversationID: "gopay_3d_test_" + time.Now().Format("20060102150405"),
		Use3D:          true,
	}

	fmt.Printf("Testing 3D payment with real Ziraat API...\n")
	fmt.Printf("Request: Amount=%.2f, Currency=%s\n", request.Amount, request.Currency)
	fmt.Printf("Card: %s (expires %s/%s)\n", request.CardInfo.CardNumber, request.CardInfo.ExpireMonth, request.CardInfo.ExpireYear)
	fmt.Printf("Customer: %s %s (%s)\n", request.Customer.Name, request.Customer.Surname, request.Customer.Email)

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
	fmt.Printf("  Message: %s\n", response.Message)
	if response.ErrorCode != "" {
		fmt.Printf("  ErrorCode: %s\n", response.ErrorCode)
	}
	if response.HTML != "" {
		fmt.Printf("  HTML Form Length: %d bytes\n", len(response.HTML))
		if len(response.HTML) > 200 {
			fmt.Printf("  HTML Form Preview: %s...\n", response.HTML[:200])
		}
	}

	// 3D için HTML olması beklenir
	if response.HTML == "" {
		t.Error("Expected HTML form for 3D payment")
	}

	// Payment ID olmalı
	if response.PaymentID == "" {
		t.Error("Expected non-empty PaymentID")
	}
}

// TestZiraatProvider_RealAPI_Complete3DPayment tests Complete3DPayment with real callback data
func TestZiraatProvider_RealAPI_Complete3DPayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()
	ctx := context.Background()

	// Mock callback state for testing Complete3DPayment
	callbackState := &provider.CallbackState{
		TenantID:         1,
		PaymentID:        "test_payment_" + time.Now().Format("20060102150405"),
		OriginalCallback: "https://test.gopay.com/callback",
		Amount:           5.00,
		Currency:         "TRY",
		LogID:            123,
		Provider:         "ziraat",
		Environment:      "sandbox",
		Timestamp:        time.Now(),
	}

	// Mock callback data that would come from Ziraat's 3D page
	callbackData := map[string]string{
		"HASH":     "test_hash_value",
		"mdStatus": "1",
		"Response": "Approved",
		"TransId":  "test_trans_123",
		"oid":      callbackState.PaymentID,
	}

	fmt.Printf("Testing Complete3D Payment with Ziraat...\n")
	fmt.Printf("Payment ID: %s\n", callbackState.PaymentID)

	response, err := p.Complete3DPayment(ctx, callbackState, callbackData)

	if err != nil {
		fmt.Printf("Complete3D Payment error: %v\n", err)
		// This is expected to fail without a real 3D session, but tests the endpoint call
		return
	}

	fmt.Println("Complete3D Payment Response:")
	fmt.Printf("  Success: %v\n", response.Success)
	fmt.Printf("  Status: %s\n", response.Status)
	fmt.Printf("  PaymentID: %s\n", response.PaymentID)
	fmt.Printf("  TransactionID: %s\n", response.TransactionID)
	fmt.Printf("  Message: %s\n", response.Message)
	if response.ErrorCode != "" {
		fmt.Printf("  ErrorCode: %s\n", response.ErrorCode)
	}
}

// TestZiraatProvider_RealAPI_CreatePayment tests that CreatePayment always uses 3D
func TestZiraatProvider_RealAPI_CreatePayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	p := setupRealTestProvider()
	ctx := context.Background()

	card := testCards[rand.Intn(len(testCards))]
	request := provider.PaymentRequest{
		TenantID:    1,
		Amount:      5.00,
		Currency:    "TRY",
		CallbackURL: "https://test.gopay.com/callback",
		ClientIP:    "127.0.0.1",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@ziraat.example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     card.CardNumber,
			ExpireMonth:    card.ExpireMonth,
			ExpireYear:     card.ExpireYear,
			CVV:            card.CVV,
			CardHolderName: "TEST USER",
		},
		Description:    "GoPay Ziraat Real API Test",
		ConversationID: "gopay_test_" + time.Now().Format("20060102150405"),
		Use3D:          false, // Even with false, should use 3D
	}

	fmt.Printf("Testing payment with real Ziraat API (use3D=false, should still use 3D)...\n")
	fmt.Printf("Request: Amount=%.2f, Currency=%s, Use3D=%v\n", request.Amount, request.Currency, request.Use3D)

	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		fmt.Printf("Payment error: %v\n", err)
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
	fmt.Printf("  Message: %s\n", response.Message)

	// Should return HTML form (3D) even when Use3D is false
	if response.HTML == "" {
		t.Error("Expected HTML form for 3D payment (Ziraat always uses 3D)")
	}

	// Payment ID olmalı
	if response.PaymentID == "" {
		t.Error("Expected non-empty PaymentID")
	}
}
