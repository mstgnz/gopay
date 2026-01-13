package ziraat

import (
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("NewProvider() returned nil")
	}

	_, ok := p.(*ZiraatProvider)
	if !ok {
		t.Fatal("NewProvider() did not return *ZiraatProvider")
	}
}

func TestGetRequiredConfig(t *testing.T) {
	p := NewProvider()
	fields := p.GetRequiredConfig("sandbox")

	if len(fields) == 0 {
		t.Fatal("GetRequiredConfig() returned no fields")
	}

	requiredKeys := map[string]bool{
		"merchantSafeId": false,
		"terminalSafeId": false,
		"secretKey":      false,
		"environment":    false,
	}

	for _, field := range fields {
		if _, exists := requiredKeys[field.Key]; exists {
			requiredKeys[field.Key] = true
		}
	}

	for key, found := range requiredKeys {
		if !found {
			t.Errorf("Required field %s not found in config", key)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]string{
				"merchantSafeId": "2025100217305644994AAC1BF57EC29B",
				"terminalSafeId": "202510021730564616275A2A52298FCF",
				"secretKey":      "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
				"environment":    "sandbox",
			},
			wantErr: false,
		},
		{
			name: "missing merchantSafeId",
			config: map[string]string{
				"terminalSafeId": "202510021730564616275A2A52298FCF",
				"secretKey":      "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
				"environment":    "sandbox",
			},
			wantErr: true,
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"merchantSafeId": "2025100217305644994AAC1BF57EC29B",
				"terminalSafeId": "202510021730564616275A2A52298FCF",
				"environment":    "sandbox",
			},
			wantErr: true,
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"merchantSafeId": "2025100217305644994AAC1BF57EC29B",
				"terminalSafeId": "202510021730564616275A2A52298FCF",
				"secretKey":      "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
				"environment":    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "valid initialization",
			config: map[string]string{
				"merchantSafeId": "2025100217305644994AAC1BF57EC29B",
				"terminalSafeId": "202510021730564616275A2A52298FCF",
				"secretKey":      "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
				"environment":    "sandbox",
			},
			wantErr: false,
		},
		{
			name: "missing credentials",
			config: map[string]string{
				"environment": "sandbox",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider()
			err := p.Initialize(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				ziraat := p.(*ZiraatProvider)
				if ziraat.merchantSafeId != tt.config["merchantSafeId"] {
					t.Errorf("merchantSafeId = %v, want %v", ziraat.merchantSafeId, tt.config["merchantSafeId"])
				}
				if ziraat.terminalSafeId != tt.config["terminalSafeId"] {
					t.Errorf("terminalSafeId = %v, want %v", ziraat.terminalSafeId, tt.config["terminalSafeId"])
				}
				if ziraat.secretKey != tt.config["secretKey"] {
					t.Errorf("secretKey = %v, want %v", ziraat.secretKey, tt.config["secretKey"])
				}
			}
		})
	}
}

func TestValidatePaymentRequest(t *testing.T) {
	p := &ZiraatProvider{}

	tests := []struct {
		name    string
		request provider.PaymentRequest
		is3D    bool
		wantErr bool
	}{
		{
			name: "valid request",
			request: provider.PaymentRequest{
				TenantID: 1,
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@test.com",
				},
				CardInfo: provider.CardInfo{
					CardNumber:  "4355084355084358",
					CVV:         "000",
					ExpireMonth: "12",
					ExpireYear:  "26",
				},
			},
			is3D:    false,
			wantErr: false,
		},
		{
			name: "missing tenant ID",
			request: provider.PaymentRequest{
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@test.com",
				},
				CardInfo: provider.CardInfo{
					CardNumber:  "4355084355084358",
					CVV:         "000",
					ExpireMonth: "12",
					ExpireYear:  "26",
				},
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "3D without callback URL",
			request: provider.PaymentRequest{
				TenantID: 1,
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@test.com",
				},
				CardInfo: provider.CardInfo{
					CardNumber:  "4355084355084358",
					CVV:         "000",
					ExpireMonth: "12",
					ExpireYear:  "26",
				},
			},
			is3D:    true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validatePaymentRequest(tt.request, tt.is3D)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAuthHash(t *testing.T) {
	p := &ZiraatProvider{
		secretKey: "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
	}

	data := `{"test":"data"}`
	hash := p.generateAuthHash(data)

	if hash == "" {
		t.Error("generateAuthHash() returned empty string")
	}

	// Test consistency
	hash2 := p.generateAuthHash(data)
	if hash != hash2 {
		t.Error("generateAuthHash() not consistent for same input")
	}

	// Test different input produces different hash
	hash3 := p.generateAuthHash(`{"test":"different"}`)
	if hash == hash3 {
		t.Error("generateAuthHash() produced same hash for different input")
	}
}

func TestGenerateRequestDateTime(t *testing.T) {
	p := &ZiraatProvider{}
	dateTime := p.generateRequestDateTime()

	if dateTime == "" {
		t.Error("generateRequestDateTime() returned empty string")
	}

	// Should contain T separator
	if !contains(dateTime, "T") {
		t.Errorf("generateRequestDateTime() = %v, want to contain T", dateTime)
	}

	// Should contain milliseconds
	if !contains(dateTime, ".") {
		t.Errorf("generateRequestDateTime() = %v, want to contain milliseconds", dateTime)
	}
}

func TestGenerateRandomNumber(t *testing.T) {
	p := &ZiraatProvider{}

	// Test different lengths
	lengths := []int{16, 32, 64, 128}
	for _, length := range lengths {
		random := p.generateRandomNumber(length)
		if len(random) != length {
			t.Errorf("generateRandomNumber(%d) returned length %d, want %d", length, len(random), length)
		}
	}

	// Test randomness (two calls should produce different results)
	random1 := p.generateRandomNumber(128)
	random2 := p.generateRandomNumber(128)
	if random1 == random2 {
		t.Error("generateRandomNumber() produced same result twice")
	}
}

func TestGenerateOrderId(t *testing.T) {
	p := &ZiraatProvider{}
	orderId := p.generateOrderId()

	if orderId == "" {
		t.Error("generateOrderId() returned empty string")
	}

	// Should start with 2 digits (year)
	if len(orderId) < 2 {
		t.Errorf("generateOrderId() too short: %s", orderId)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
