package postgres

import (
	"encoding/json"
	"strings"
	"testing"
)

// productionLikePayload mirrors the shape actually stored in the paycell table for a
// /payment/3d request, so the test fails if the real payload stops being covered.
func productionLikePayload() map[string]any {
	return map[string]any{
		"amount":   134436.4,
		"use3D":    true,
		"currency": "TRY",
		"cardInfo": map[string]any{
			"cvv":            "408",
			"cardNumber":     "6501234567890559",
			"expireYear":     "2032",
			"expireMonth":    "05",
			"cardHolderName": "HUSEYIN UBEYT",
		},
		"cardTokenRequest": map[string]any{
			"cvcNo":          "408",
			"creditCardNo":   "6501234567890559",
			"expireDateYear": "32",
			"header": map[string]any{
				"applicationPwd":  "A092Z8QZ3N72G1PH",
				"applicationName": "SOVTAJYERI",
				"transactionId":   "66620260718154139842",
			},
		},
		"cardTokenResponse": map[string]any{
			"cardToken": "47091d4b-5794-4245-ba10-aab1f995f164",
		},
		"getThreeDSessionRequest": map[string]any{
			"msisdn":       "5320698039",
			"cardToken":    "47091d4b-5794-4245-ba10-aab1f995f164",
			"merchantCode": "237431",
			"amount":       "13443640",
		},
		"providerProvisionRequest": map[string]any{
			"referenceNumber": "0155517501467149523",
			"amount":          "13443640",
		},
	}
}

func TestSanitizeForLogRedactsCVVAndCredentials(t *testing.T) {
	got := SanitizeForLog(productionLikePayload())

	cardInfo := got["cardInfo"].(map[string]any)
	if cardInfo["cvv"] != "***" {
		t.Errorf("cardInfo.cvv = %v, want ***", cardInfo["cvv"])
	}
	if cardInfo["cardNumber"] != "6501********0559" {
		t.Errorf("cardInfo.cardNumber = %v, want 6501********0559", cardInfo["cardNumber"])
	}

	tokenReq := got["cardTokenRequest"].(map[string]any)
	if tokenReq["cvcNo"] != "***" {
		t.Errorf("cardTokenRequest.cvcNo = %v, want ***", tokenReq["cvcNo"])
	}

	header := tokenReq["header"].(map[string]any)
	if header["applicationPwd"] != "***REDACTED***" {
		t.Errorf("applicationPwd = %v, want ***REDACTED***", header["applicationPwd"])
	}
	if header["applicationName"] != "SOVTAJYERI" {
		t.Errorf("applicationName was altered: %v", header["applicationName"])
	}
}

// TestSanitizeForLogPreservesReplayedFields guards the fields that GetPaymentStatus and
// Complete3DPayment read back out of the log. Masking any of these breaks live payments,
// so this test is the regression fence for the sanitize patterns.
func TestSanitizeForLogPreservesReplayedFields(t *testing.T) {
	got := SanitizeForLog(productionLikePayload())

	if got["amount"] != 134436.4 {
		t.Errorf("amount = %v, want 134436.4", got["amount"])
	}

	tokenResp := got["cardTokenResponse"].(map[string]any)
	if tokenResp["cardToken"] != "47091d4b-5794-4245-ba10-aab1f995f164" {
		t.Errorf("cardToken was masked: %v", tokenResp["cardToken"])
	}

	sessionReq := got["getThreeDSessionRequest"].(map[string]any)
	if sessionReq["cardToken"] != "47091d4b-5794-4245-ba10-aab1f995f164" {
		t.Errorf("getThreeDSessionRequest.cardToken was masked: %v", sessionReq["cardToken"])
	}
	if sessionReq["msisdn"] != "5320698039" {
		t.Errorf("msisdn was masked: %v", sessionReq["msisdn"])
	}

	provision := got["providerProvisionRequest"].(map[string]any)
	if provision["referenceNumber"] != "0155517501467149523" {
		t.Errorf("referenceNumber was masked: %v", provision["referenceNumber"])
	}
}

// TestSanitizeForLogDoesNotMutateInput proves the KVKK split: the DB copy is masked while
// the caller's payload, which is what actually goes to the provider, stays untouched.
func TestSanitizeForLogDoesNotMutateInput(t *testing.T) {
	original := productionLikePayload()
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	_ = SanitizeForLog(original)

	after, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("SanitizeForLog mutated its input\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestSanitizeForLogNoCleartextSecrets is the blunt end-to-end assertion: no secret value
// from the payload may survive anywhere in the serialized log record.
func TestSanitizeForLogNoCleartextSecrets(t *testing.T) {
	encoded, err := json.Marshal(SanitizeForLog(productionLikePayload()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	serialized := string(encoded)

	for _, secret := range []string{"A092Z8QZ3N72G1PH", "6501234567890559", "\"408\""} {
		if strings.Contains(serialized, secret) {
			t.Errorf("sanitized log still contains %q: %s", secret, serialized)
		}
	}
}
