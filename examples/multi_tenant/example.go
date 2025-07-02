package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	// GoPay server base URL
	baseURL = "http://localhost:9999/v1"
)

// PaymentStatus represents the current status of a payment
type PaymentStatus string

const (
	StatusPending    PaymentStatus = "pending"
	StatusProcessing PaymentStatus = "processing"
	StatusSuccessful PaymentStatus = "successful"
	StatusFailed     PaymentStatus = "failed"
	StatusCancelled  PaymentStatus = "cancelled"
	StatusRefunded   PaymentStatus = "refunded"
)

// Address represents a physical address
type Address struct {
	City        string `json:"city"`
	Country     string `json:"country"`
	Address     string `json:"address"`
	ZipCode     string `json:"zipCode"`
	Description string `json:"description,omitempty"`
}

// Customer represents the buyer information
type Customer struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Surname     string  `json:"surname"`
	Email       string  `json:"email"`
	PhoneNumber string  `json:"phoneNumber,omitempty"`
	IPAddress   string  `json:"ipAddress,omitempty"`
	Address     Address `json:"address,omitempty"`
}

// CardInfo represents credit card information
type CardInfo struct {
	CardHolderName string `json:"cardHolderName"`
	CardNumber     string `json:"cardNumber"`
	ExpireMonth    string `json:"expireMonth"`
	ExpireYear     string `json:"expireYear"`
	CVV            string `json:"cvv"`
}

// Item represents a product or service item in the payment
type Item struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category,omitempty"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

// PaymentRequest contains all information required to create a payment
type PaymentRequest struct {
	ID               string   `json:"id,omitempty"`
	ReferenceID      string   `json:"referenceId,omitempty"`
	Currency         string   `json:"currency"`
	Amount           float64  `json:"amount"`
	Customer         Customer `json:"customer"`
	CardInfo         CardInfo `json:"cardInfo"`
	Items            []Item   `json:"items,omitempty"`
	Description      string   `json:"description,omitempty"`
	CallbackURL      string   `json:"callbackUrl,omitempty"`
	Use3D            bool     `json:"use3D"`
	InstallmentCount int      `json:"installmentCount,omitempty"`
	PaymentChannel   string   `json:"paymentChannel,omitempty"`
	PaymentGroup     string   `json:"paymentGroup,omitempty"`
	ConversationID   string   `json:"conversationId,omitempty"`
	Locale           string   `json:"locale,omitempty"`
	ClientIP         string   `json:"clientIp,omitempty"`
	ClientUserAgent  string   `json:"clientUserAgent,omitempty"`
	MetaData         string   `json:"metaData,omitempty"`
	RedirectURL      string   `json:"redirectUrl,omitempty"`
}

// PaymentResponse contains the result of a payment request
type PaymentResponse struct {
	Success          bool          `json:"success"`
	Status           PaymentStatus `json:"status"`
	Message          string        `json:"message,omitempty"`
	ErrorCode        string        `json:"errorCode,omitempty"`
	TransactionID    string        `json:"transactionId,omitempty"`
	PaymentID        string        `json:"paymentId,omitempty"`
	Amount           float64       `json:"amount"`
	Currency         string        `json:"currency"`
	RedirectURL      string        `json:"redirectUrl,omitempty"`
	HTML             string        `json:"html,omitempty"`
	SystemTime       time.Time     `json:"systemTime,omitempty"`
	FraudStatus      int           `json:"fraudStatus,omitempty"`
	ProviderResponse any           `json:"providerResponse,omitempty"`
}

// MultiTenantClient represents a client for multi-tenant payment operations
type MultiTenantClient struct {
	baseURL    string
	httpClient *http.Client
}

// TenantConfig represents the configuration for a tenant
type TenantConfig struct {
	TenantID string `json:"tenantId"`

	// İyzico configuration
	IyzicoApiKey    string `json:"IYZICO_API_KEY,omitempty"`
	IyzicoSecretKey string `json:"IYZICO_SECRET_KEY,omitempty"`
	IyzicoEnv       string `json:"IYZICO_ENVIRONMENT,omitempty"`

	// OzanPay configuration
	OzanpayApiKey    string `json:"OZANPAY_API_KEY,omitempty"`
	OzanpaySecretKey string `json:"OZANPAY_SECRET_KEY,omitempty"`
	OzanpayMerchant  string `json:"OZANPAY_MERCHANT_ID,omitempty"`
	OzanpayEnv       string `json:"OZANPAY_ENVIRONMENT,omitempty"`

	// Paycell configuration
	PaycellUsername   string `json:"PAYCELL_USERNAME,omitempty"`
	PaycellPassword   string `json:"PAYCELL_PASSWORD,omitempty"`
	PaycellMerchantId string `json:"PAYCELL_MERCHANT_ID,omitempty"`
	PaycellTerminalId string `json:"PAYCELL_TERMINAL_ID,omitempty"`
	PaycellEnv        string `json:"PAYCELL_ENVIRONMENT,omitempty"`
}

// NewMultiTenantClient creates a new multi-tenant client
func NewMultiTenantClient(baseURL string) *MultiTenantClient {
	return &MultiTenantClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetupTenant configures a tenant with provider credentials
func (c *MultiTenantClient) SetupTenant(config TenantConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/set-env", bytes.NewBuffer(configJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", config.TenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("setup failed with status: %d", resp.StatusCode)
	}

	return nil
}

// CreatePayment creates a payment for a specific tenant
func (c *MultiTenantClient) CreatePayment(tenantID, provider string, paymentReq PaymentRequest) (*PaymentResponse, error) {
	paymentJSON, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %w", err)
	}

	url := fmt.Sprintf("%s/payments/%s", c.baseURL, provider)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(paymentJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &paymentResp, nil
}

// GetPaymentStatus gets payment status for a specific tenant
func (c *MultiTenantClient) GetPaymentStatus(tenantID, provider, paymentID string) (*PaymentResponse, error) {
	url := fmt.Sprintf("%s/payments/%s/%s", c.baseURL, provider, paymentID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &paymentResp, nil
}

func main() {
	fmt.Println("🏢 Multi-Tenant GoPay Example")
	fmt.Println("=============================")

	// Create multi-tenant client
	client := NewMultiTenantClient(baseURL)

	// Setup multiple tenants
	fmt.Println("\n1. Setting up tenants...")
	setupTenants(client)

	// Example payments for different tenants
	fmt.Println("\n2. Processing payments for different tenants...")
	processPayments(client)

	fmt.Println("\n✅ Multi-tenant example completed!")
	fmt.Println("\nTo run this example:")
	fmt.Println("1. Start your GoPay server: go run cmd/main.go")
	fmt.Println("2. Run this example: go run examples/multi_tenant/example.go")
}

func setupTenants(client *MultiTenantClient) {
	tenants := []TenantConfig{
		{
			TenantID:        "ABC",
			IyzicoApiKey:    "sandbox-abc-iyzico-api-key",
			IyzicoSecretKey: "sandbox-abc-iyzico-secret-key",
			IyzicoEnv:       "sandbox",
		},
		{
			TenantID:        "DEF",
			IyzicoApiKey:    "sandbox-def-iyzico-api-key",
			IyzicoSecretKey: "sandbox-def-iyzico-secret-key",
			IyzicoEnv:       "sandbox",
		},
		{
			TenantID:         "XYZ",
			OzanpayApiKey:    "xyz-ozanpay-api-key",
			OzanpaySecretKey: "xyz-ozanpay-secret-key",
			OzanpayMerchant:  "xyz-merchant-12345",
			OzanpayEnv:       "sandbox",
		},
		{
			TenantID:          "ENTERPRISE",
			IyzicoApiKey:      "enterprise-iyzico-api-key",
			IyzicoSecretKey:   "enterprise-iyzico-secret-key",
			IyzicoEnv:         "production",
			PaycellUsername:   "enterprise-paycell-user",
			PaycellPassword:   "enterprise-paycell-pass",
			PaycellMerchantId: "enterprise-merchant-789",
			PaycellTerminalId: "enterprise-terminal-456",
			PaycellEnv:        "production",
		},
	}

	for _, tenant := range tenants {
		fmt.Printf("   Setting up tenant: %s\n", tenant.TenantID)
		if err := client.SetupTenant(tenant); err != nil {
			log.Printf("Failed to setup tenant %s: %v", tenant.TenantID, err)
		} else {
			fmt.Printf("   ✅ Tenant %s configured successfully\n", tenant.TenantID)
		}
	}
}

func processPayments(client *MultiTenantClient) {
	// ABC Tenant Payment (İyzico)
	fmt.Println("   Processing ABC tenant payment (İyzico)...")
	abcPayment := PaymentRequest{
		Amount:   150.00,
		Currency: "TRY",
		Customer: Customer{
			ID:      "abc_customer_001",
			Name:    "Ahmet",
			Surname: "Yılmaz",
			Email:   "ahmet@abc-company.com",
			Address: Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "ABC Company Address",
				ZipCode: "34000",
			},
		},
		CardInfo: CardInfo{
			CardHolderName: "Ahmet Yılmaz",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Items: []Item{
			{
				ID:       "abc_product_1",
				Name:     "ABC Product",
				Category: "Electronics",
				Price:    150.00,
				Quantity: 1,
			},
		},
		Description:    "ABC Tenant payment via İyzico",
		Use3D:          false,
		ConversationID: "abc_conv_001",
	}

	abcResp, err := client.CreatePayment("ABC", "iyzico", abcPayment)
	if err != nil {
		log.Printf("ABC payment failed: %v", err)
	} else if abcResp.Success {
		fmt.Printf("   ✅ ABC payment successful (ID: %s)\n", abcResp.PaymentID)
	} else {
		fmt.Printf("   ❌ ABC payment failed: %s\n", abcResp.Message)
	}

	// XYZ Tenant Payment (OzanPay)
	fmt.Println("   Processing XYZ tenant payment (OzanPay)...")
	xyzPayment := PaymentRequest{
		Amount:   320.75,
		Currency: "TRY",
		Customer: Customer{
			ID:      "xyz_customer_vip",
			Name:    "Mehmet",
			Surname: "Özkan",
			Email:   "mehmet@xyz-solutions.com",
			Address: Address{
				City:    "İzmir",
				Country: "Turkey",
				Address: "XYZ Solutions Office",
				ZipCode: "35000",
			},
		},
		CardInfo: CardInfo{
			CardHolderName: "Mehmet Özkan",
			CardNumber:     "4111111111111111",
			ExpireMonth:    "10",
			ExpireYear:     "2028",
			CVV:            "789",
		},
		Items: []Item{
			{
				ID:       "xyz_solution_package",
				Name:     "XYZ Solution Package",
				Category: "Software",
				Price:    320.75,
				Quantity: 1,
			},
		},
		Description:    "XYZ Tenant solution package purchase",
		Use3D:          false,
		ConversationID: "xyz_conv_vip_001",
	}

	xyzResp, err := client.CreatePayment("XYZ", "ozanpay", xyzPayment)
	if err != nil {
		log.Printf("XYZ payment failed: %v", err)
	} else if xyzResp.Success {
		fmt.Printf("   ✅ XYZ payment successful (ID: %s)\n", xyzResp.PaymentID)
	} else {
		fmt.Printf("   ❌ XYZ payment failed: %s\n", xyzResp.Message)
	}

	// ENTERPRISE Tenant 3D Payment (İyzico)
	fmt.Println("   Processing ENTERPRISE tenant 3D payment (İyzico)...")
	enterprisePayment := PaymentRequest{
		Amount:   1500.00,
		Currency: "TRY",
		Customer: Customer{
			ID:      "enterprise_ceo",
			Name:    "Ali",
			Surname: "Kaya",
			Email:   "ali.kaya@enterprise-corp.com",
			Address: Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Enterprise Corp Tower",
				ZipCode: "34200",
			},
		},
		CardInfo: CardInfo{
			CardHolderName: "Ali Kaya",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "09",
			ExpireYear:     "2027",
			CVV:            "321",
		},
		Items: []Item{
			{
				ID:       "enterprise_license",
				Name:     "Enterprise License",
				Category: "License",
				Price:    1500.00,
				Quantity: 1,
			},
		},
		Description:    "Enterprise annual license payment",
		Use3D:          true,
		CallbackURL:    "https://enterprise-corp.com/payment-callback",
		ConversationID: "enterprise_license_2024",
	}

	enterpriseResp, err := client.CreatePayment("ENTERPRISE", "iyzico", enterprisePayment)
	if err != nil {
		log.Printf("ENTERPRISE payment failed: %v", err)
	} else if enterpriseResp.Status == StatusPending {
		fmt.Printf("   🔐 ENTERPRISE 3D payment initiated (ID: %s)\n", enterpriseResp.PaymentID)
		if enterpriseResp.HTML != "" {
			fmt.Printf("   📄 3D HTML form received (length: %d)\n", len(enterpriseResp.HTML))
		}
		if enterpriseResp.RedirectURL != "" {
			fmt.Printf("   🔗 Redirect URL: %s\n", enterpriseResp.RedirectURL)
		}
	} else {
		fmt.Printf("   ❌ ENTERPRISE payment failed: %s\n", enterpriseResp.Message)
	}
}
