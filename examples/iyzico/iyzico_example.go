package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mstgnz/gopay/provider"
	_ "github.com/mstgnz/gopay/provider/iyzico" // Import for side-effect registration
)

func main() {
	// Create payment service
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})

	// Configure İyzico provider
	iyzicoConfig := map[string]string{
		"apiKey":      "your-iyzico-api-key",
		"secretKey":   "your-iyzico-secret-key",
		"environment": "sandbox", // or "production"
	}

	// Add İyzico provider
	err := paymentService.AddProvider("iyzico", iyzicoConfig)
	if err != nil {
		log.Fatalf("Failed to add İyzico provider: %v", err)
	}

	// Set as default provider
	err = paymentService.SetDefaultProvider("iyzico")
	if err != nil {
		log.Fatalf("Failed to set default provider: %v", err)
	}

	// Example 1: Regular Payment
	fmt.Println("=== Regular Payment Example ===")
	regularPaymentExample(paymentService)

	// Example 2: 3D Secure Payment
	fmt.Println("\n=== 3D Secure Payment Example ===")
	threeDPaymentExample(paymentService)

	// Example 3: Payment Status Check
	fmt.Println("\n=== Payment Status Check Example ===")
	paymentStatusExample(paymentService)

	// Example 4: Refund Payment
	fmt.Println("\n=== Refund Payment Example ===")
	refundPaymentExample(paymentService)
}

func regularPaymentExample(paymentService *provider.PaymentService) {
	// Create payment request
	paymentRequest := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:      "customer123",
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
			Address: &provider.Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "5528790000000008", // İyzico test card
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Items: []provider.Item{
			{
				ID:       "item1",
				Name:     "Test Product",
				Category: "Electronics",
				Price:    100.50,
				Quantity: 1,
			},
		},
		Description: "Test payment via İyzico",
		Use3D:       false,
	}

	// Process payment
	ctx := context.Background()
	response, err := paymentService.CreatePayment(ctx, "iyzico", paymentRequest)
	if err != nil {
		log.Printf("Payment failed: %v", err)
		return
	}

	// Check payment result
	if response.Success {
		fmt.Printf("✅ Payment successful!\n")
		fmt.Printf("   Payment ID: %s\n", response.PaymentID)
		fmt.Printf("   Transaction ID: %s\n", response.TransactionID)
		fmt.Printf("   Amount: %.2f %s\n", response.Amount, response.Currency)
		fmt.Printf("   Status: %s\n", response.Status)
	} else {
		fmt.Printf("❌ Payment failed!\n")
		fmt.Printf("   Error: %s\n", response.Message)
		fmt.Printf("   Error Code: %s\n", response.ErrorCode)
	}
}

func threeDPaymentExample(paymentService *provider.PaymentService) {
	// Create 3D secure payment request
	paymentRequest := provider.PaymentRequest{
		Amount:   250.00,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:      "customer456",
			Name:    "Jane",
			Surname: "Smith",
			Email:   "jane@example.com",
			Address: &provider.Address{
				City:    "Ankara",
				Country: "Turkey",
				Address: "Test Address 456",
				ZipCode: "06000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Jane Smith",
			CardNumber:     "5528790000000008", // İyzico test card
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Items: []provider.Item{
			{
				ID:       "item2",
				Name:     "Premium Product",
				Category: "Premium",
				Price:    250.00,
				Quantity: 1,
			},
		},
		Description: "3D Secure test payment via İyzico",
		Use3D:       true,
		CallbackURL: "https://your-website.com/payment-callback",
	}

	// Process 3D payment
	ctx := context.Background()
	response, err := paymentService.CreatePayment(ctx, "iyzico", paymentRequest)
	if err != nil {
		log.Printf("3D Payment initiation failed: %v", err)
		return
	}

	if response.Status == provider.StatusPending {
		fmt.Printf("🔐 3D Secure authentication required\n")
		fmt.Printf("   Payment ID: %s\n", response.PaymentID)
		if response.HTML != "" {
			fmt.Printf("   HTML form received (length: %d characters)\n", len(response.HTML))
			fmt.Printf("   💡 In a real application, show this HTML to the user\n")
		}
		if response.RedirectURL != "" {
			fmt.Printf("   Redirect URL: %s\n", response.RedirectURL)
			fmt.Printf("   💡 In a real application, redirect user to this URL\n")
		}

		// Simulate 3D completion (in real scenario, this comes from callback)
		fmt.Printf("\n   📝 Simulating 3D completion...\n")
		completeResponse, err := paymentService.Complete3DPayment(
			ctx,
			"iyzico",
			response.PaymentID,
			"conv123",
			map[string]string{
				"status":   "success",
				"mdStatus": "1",
			},
		)

		if err != nil {
			log.Printf("3D completion failed: %v", err)
			return
		}

		if completeResponse.Success {
			fmt.Printf("   ✅ 3D Payment completed successfully!\n")
			fmt.Printf("   Payment ID: %s\n", completeResponse.PaymentID)
			fmt.Printf("   Transaction ID: %s\n", completeResponse.TransactionID)
		} else {
			fmt.Printf("   ❌ 3D Payment completion failed!\n")
			fmt.Printf("   Error: %s\n", completeResponse.Message)
		}
	}
}

func paymentStatusExample(paymentService *provider.PaymentService) {
	// Check payment status (use a payment ID from previous examples)
	paymentID := "example-payment-id-123"

	ctx := context.Background()
	response, err := paymentService.GetPaymentStatus(ctx, "iyzico", paymentID)
	if err != nil {
		log.Printf("Status check failed: %v", err)
		return
	}

	fmt.Printf("📊 Payment Status Check\n")
	fmt.Printf("   Payment ID: %s\n", paymentID)
	if response.Success {
		fmt.Printf("   Status: %s\n", response.Status)
		fmt.Printf("   Amount: %.2f %s\n", response.Amount, response.Currency)
		fmt.Printf("   ✅ Payment found and verified\n")
	} else {
		fmt.Printf("   ❌ Payment not found or failed\n")
		fmt.Printf("   Error: %s\n", response.Message)
	}
}

func refundPaymentExample(paymentService *provider.PaymentService) {
	// Create refund request
	refundRequest := provider.RefundRequest{
		PaymentID:    "example-payment-id-123",
		RefundAmount: 50.00, // Partial refund
		Reason:       "Customer request",
		Description:  "Customer requested partial refund",
		Currency:     "TRY",
	}

	ctx := context.Background()
	response, err := paymentService.RefundPayment(ctx, "iyzico", refundRequest)
	if err != nil {
		log.Printf("Refund failed: %v", err)
		return
	}

	fmt.Printf("💰 Refund Process\n")
	if response.Success {
		fmt.Printf("   ✅ Refund successful!\n")
		fmt.Printf("   Refund ID: %s\n", response.RefundID)
		fmt.Printf("   Payment ID: %s\n", response.PaymentID)
		fmt.Printf("   Refund Amount: %.2f\n", response.RefundAmount)
		fmt.Printf("   Status: %s\n", response.Status)
	} else {
		fmt.Printf("   ❌ Refund failed!\n")
		fmt.Printf("   Error: %s\n", response.Message)
		fmt.Printf("   Error Code: %s\n", response.ErrorCode)
	}
}

// Test card numbers for İyzico sandbox:
// Successful: 5528790000000008
// Insufficient funds: 5528790000000016
// Do not honor: 5528790000000024
// Invalid card: 5528790000000032
// Lost card: 5528790000000040
// Stolen card: 5528790000000057
// Expired card: 5528790000000065
// Invalid security code: 5528790000000073
// Invalid amount: 5528790000000081
// General error: 5528790000000099

// 3D Test Cards:
// Successful 3D: 5528790000000008
// Failed 3D: 5528790000000016
