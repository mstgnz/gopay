package postgres

import (
	"encoding/json"
	"strings"
	"testing"
)

// Fixture values are synthetic (a public Paycell test card, made-up credentials, msisdn,
// references and amounts).
// Never paste production log rows here: this repository is public.
const (
	testPAN            = "4355084355084358"
	testCVV            = "913"
	testApplicationPwd = "FIXTUREPWD000001"
	testMSISDN         = "5000000001"
)

// productionLikePayload mirrors the shape actually stored in the paycell table for a
// /payment/3d request, so the test fails if the real payload stops being covered.
func productionLikePayload() map[string]any {
	return map[string]any{
		"amount":   1234.5,
		"use3D":    true,
		"currency": "TRY",
		"cardInfo": map[string]any{
			"cvv":            testCVV,
			"cardNumber":     testPAN,
			"expireYear":     "2032",
			"expireMonth":    "05",
			"cardHolderName": "TEST CARDHOLDER",
		},
		"cardTokenRequest": map[string]any{
			"cvcNo":          testCVV,
			"creditCardNo":   testPAN,
			"expireDateYear": "32",
			"header": map[string]any{
				"applicationPwd":  testApplicationPwd,
				"applicationName": "FIXTUREAPP",
				"transactionId":   "00000000000000000001",
			},
		},
		"cardTokenResponse": map[string]any{
			"cardToken": "00000000-0000-4000-8000-000000000001",
		},
		"getThreeDSessionRequest": map[string]any{
			"msisdn":       testMSISDN,
			"cardToken":    "00000000-0000-4000-8000-000000000001",
			"merchantCode": "999999",
			"amount":       "123450",
		},
		"providerProvisionRequest": map[string]any{
			"referenceNumber": "0000000000000000002",
			"amount":          "123450",
		},
	}
}

func TestSanitizeForLogRedactsCVVAndCredentials(t *testing.T) {
	got := SanitizeForLog(productionLikePayload())

	cardInfo := got["cardInfo"].(map[string]any)
	if cardInfo["cvv"] != "***" {
		t.Errorf("cardInfo.cvv = %v, want ***", cardInfo["cvv"])
	}
	if cardInfo["cardNumber"] != "4355********4358" {
		t.Errorf("cardInfo.cardNumber = %v, want 4355********4358", cardInfo["cardNumber"])
	}

	tokenReq := got["cardTokenRequest"].(map[string]any)
	if tokenReq["cvcNo"] != "***" {
		t.Errorf("cardTokenRequest.cvcNo = %v, want ***", tokenReq["cvcNo"])
	}

	header := tokenReq["header"].(map[string]any)
	if header["applicationPwd"] != "***REDACTED***" {
		t.Errorf("applicationPwd = %v, want ***REDACTED***", header["applicationPwd"])
	}
	if header["applicationName"] != "FIXTUREAPP" {
		t.Errorf("applicationName was altered: %v", header["applicationName"])
	}
}

// TestSanitizeForLogPreservesReplayedFields guards the fields that GetPaymentStatus and
// Complete3DPayment read back out of the log. Masking any of these breaks live payments,
// so this test is the regression fence for the sanitize patterns.
func TestSanitizeForLogPreservesReplayedFields(t *testing.T) {
	got := SanitizeForLog(productionLikePayload())

	if got["amount"] != 1234.5 {
		t.Errorf("amount = %v, want 1234.5", got["amount"])
	}

	tokenResp := got["cardTokenResponse"].(map[string]any)
	if tokenResp["cardToken"] != "00000000-0000-4000-8000-000000000001" {
		t.Errorf("cardToken was masked: %v", tokenResp["cardToken"])
	}

	sessionReq := got["getThreeDSessionRequest"].(map[string]any)
	if sessionReq["cardToken"] != "00000000-0000-4000-8000-000000000001" {
		t.Errorf("getThreeDSessionRequest.cardToken was masked: %v", sessionReq["cardToken"])
	}
	if sessionReq["msisdn"] != testMSISDN {
		t.Errorf("msisdn was masked: %v", sessionReq["msisdn"])
	}

	provision := got["providerProvisionRequest"].(map[string]any)
	if provision["referenceNumber"] != "0000000000000000002" {
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

// paycellHeader mirrors the provider header structs that are embedded as Go values inside
// the request maps handed to AddProviderRequestToClientRequest.
type paycellHeader struct {
	TransactionID       string `json:"transactionId"`
	TransactionDateTime string `json:"transactionDateTime"`
	ClientIPAddress     string `json:"clientIPAddress"`
	ApplicationName     string `json:"applicationName"`
	ApplicationPwd      string `json:"applicationPwd"`
}

// TestSanitizeForLogRedactsInsideNestedStructs pins the regression that shipped to
// production on 2026-07-18: the fixture must hold a real struct, not a map decoded from an
// already-logged payload. sanitizeRecursive walks only maps and slices, so a struct value
// slipped through and its applicationPwd was written to the provider table in cleartext.
func TestSanitizeForLogRedactsInsideNestedStructs(t *testing.T) {
	got := SanitizeForLog(map[string]any{
		"msisdn":                  testMSISDN,
		"merchantCode":            "999999",
		"originalReferenceNumber": "00000000000000000003",
		"requestHeader": paycellHeader{
			TransactionID:       "00000000000000000004",
			TransactionDateTime: "20260101000000000",
			ClientIPAddress:     "127.0.0.1",
			ApplicationName:     "FIXTUREAPP",
			ApplicationPwd:      testApplicationPwd,
		},
	})

	header, ok := got["requestHeader"].(map[string]any)
	if !ok {
		t.Fatalf("requestHeader was not normalized into a map: %T", got["requestHeader"])
	}
	if header["applicationPwd"] != "***REDACTED***" {
		t.Errorf("applicationPwd = %v, want ***REDACTED***", header["applicationPwd"])
	}
	if header["applicationName"] != "FIXTUREAPP" {
		t.Errorf("applicationName was altered: %v", header["applicationName"])
	}
	if got["msisdn"] != testMSISDN {
		t.Errorf("msisdn was altered: %v", got["msisdn"])
	}
	if got["originalReferenceNumber"] != "00000000000000000003" {
		t.Errorf("originalReferenceNumber was altered: %v", got["originalReferenceNumber"])
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

	for _, secret := range []string{testApplicationPwd, testPAN, `"` + testCVV + `"`} {
		if strings.Contains(serialized, secret) {
			t.Errorf("sanitized log still contains %q: %s", secret, serialized)
		}
	}
}

func TestSanitizeForLogRedactsNkolaySx(t *testing.T) {
	got := SanitizeForLog(map[string]any{
		"sx":            "118591467|fixture-token",
		"sxList":        "118591467|fixture-token|list",
		"sxCancel":      "118591467|fixture-token|cancel",
		"clientRefCode": "gopay_1",
		"systemTime":    "2025-07-23T11:30:21.163704+03",
	})

	for _, key := range []string{"sx", "sxList", "sxCancel"} {
		if got[key] != "***REDACTED***" {
			t.Errorf("%s = %v, want ***REDACTED***", key, got[key])
		}
	}
	// nkolay cancel and refund read systemTime back out of the log.
	if got["systemTime"] != "2025-07-23T11:30:21.163704+03" || got["clientRefCode"] != "gopay_1" {
		t.Errorf("non-secret fields altered: %+v", got)
	}
}

// Nkolay answers a 3D request with an auto-submitting form that echoes the card. It reaches the
// log as a string, either as plain HTML or inside a raw JSON body with escaped quotes.
func TestSanitizeForLogRedactsCardInputsInHTML(t *testing.T) {
	plain := `<form action="https://bank.example/3d" method="post">` +
		`<input type="hidden" name="pan" value="` + testPAN + `">` +
		`<input type="hidden" name="cv2" value="` + testCVV + `"/>` +
		`<input type="hidden" value="` + testPAN + `" name="cardNumber">` +
		`<INPUT TYPE="hidden" NAME="CVV" VALUE='` + testCVV + `'>` +
		`<input type="hidden" name="CardCVV2" value="` + testCVV + `">` +
		`<input type="hidden" value="` + testCVV + `" name="cvc2">` +
		`<input type="hidden" name="successUrl" value="https://payment.example/v1/callback/nkolay?state=7">` +
		`<input type="hidden" name="panelId" value="keep-me">` +
		`</form>`
	escaped := strings.ReplaceAll(plain, `"`, `\"`)
	rawBody := `{"RESPONSE_CODE":2,"BANK_REQUEST_MESSAGE":"` + escaped + `"}`

	got := SanitizeForLog(map[string]any{
		"html":    plain,
		"message": plain,
		"providerResponse": map[string]any{
			"raw_response": rawBody,
		},
		"items": []any{plain},
	})

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	serialized := string(encoded)
	for _, secret := range []string{testPAN, testCVV} {
		if strings.Contains(serialized, secret) {
			t.Errorf("card data survived in the logged HTML: %q\n%s", secret, serialized)
		}
	}
	for _, kept := range []string{"https://payment.example/v1/callback/nkolay?state=7", "keep-me", "https://bank.example/3d"} {
		if !strings.Contains(serialized, kept) {
			t.Errorf("non-card form content was removed: %q", kept)
		}
	}
}

func TestRedactCardInputsLeavesOrdinaryStringsAlone(t *testing.T) {
	for _, s := range []string{
		"",
		"gopay_1739000000",
		"Payment input was invalid",
		`<input name="panel" value="x">`,
		`<input name="spanish" value="x">`,
	} {
		if got := redactCardInputs(s); got != s {
			t.Errorf("redactCardInputs(%q) = %q, want unchanged", s, got)
		}
	}
}
