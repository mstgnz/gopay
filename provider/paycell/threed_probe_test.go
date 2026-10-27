package paycell

import (
	"context"
	"os"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

// TestProbeThreeDSessionResult sends one getThreeDSessionResult for a 3D session and prints Paycell's
// raw answer. It is a read: no provision, no money moves. It shows what Paycell answers for a session the
// payer never completed, before the status expiry rule relies on that answer. Skipped unless every
// PAYCELL_PROBE_* variable is set; credentials come from the environment and are never printed:
//
//	PAYCELL_PROBE_ENV=production PAYCELL_PROBE_USERNAME=... PAYCELL_PROBE_PASSWORD=... \
//	PAYCELL_PROBE_MERCHANT_ID=... PAYCELL_PROBE_SECURE_CODE=... PAYCELL_PROBE_MSISDN=... \
//	PAYCELL_PROBE_SESSION_ID=... go test -run ProbeThreeDSessionResult -v ./provider/paycell/
func TestProbeThreeDSessionResult(t *testing.T) {
	env := map[string]string{}
	for _, key := range []string{"ENV", "USERNAME", "PASSWORD", "MERCHANT_ID", "SECURE_CODE", "MSISDN", "SESSION_ID"} {
		env[key] = os.Getenv("PAYCELL_PROBE_" + key)
		if env[key] == "" {
			t.Skip("set every PAYCELL_PROBE_* variable to run this probe")
		}
	}

	p := NewProvider().(*PaycellProvider)
	if err := p.Initialize(map[string]string{
		"username":    env["USERNAME"],
		"password":    env["PASSWORD"],
		"merchantId":  env["MERCHANT_ID"],
		"secureCode":  env["SECURE_CODE"],
		"environment": env["ENV"],
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Same body as threeDSessionResult, sent directly so the raw answer is printed, including any field
	// PaycellGetThreeDSessionResultResponse does not decode.
	resp, err := p.httpClient.SendJSON(context.Background(), &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: endpointGetThreeDSessionResult,
		Body: map[string]any{
			"merchantCode":    p.merchantID,
			"msisdn":          env["MSISDN"],
			"threeDSessionId": env["SESSION_ID"],
			"requestHeader": PaycellRequestHeader{
				TransactionID:       p.generateTransactionID(),
				TransactionDateTime: p.generateTransactionDateTime(),
				ClientIPAddress:     "127.0.0.1",
				ApplicationName:     p.username,
				ApplicationPwd:      p.password,
			},
		},
	})
	if err != nil {
		t.Fatalf("getThreeDSessionResult: %v", err)
	}
	t.Logf("environment %s, session %s, HTTP %d\n%s", env["ENV"], env["SESSION_ID"], resp.StatusCode, resp.RawBody)
}
