package paycell

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

var _ provider.RefundLedgerReader = (*PaycellProvider)(nil)

func ledgerFrom(t *testing.T, inquire PaycellInquireResponse) (provider.RefundLedger, *logRecorder, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointInquireAll {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(inquire)
	}))
	t.Cleanup(srv.Close)
	rec := newLogRecorder()
	p := bindingProvider(t, srv.URL, rec)
	led, err := p.RefundLedger(context.Background(), "pay-1", 88)
	return led, rec, err
}

func provision(kind, code, amount string) PaycellProvisionListItem {
	return PaycellProvisionListItem{ProvisionType: kind, ResponseCode: code, Amount: amount}
}

func TestRefundLedgerSumsSuccessfulRefunds(t *testing.T) {
	led, rec, err := ledgerFrom(t, PaycellInquireResponse{
		ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
		ProvisionList: []PaycellProvisionListItem{
			provision("SALE", responseCodeSuccess, "50000"),
			provision("REFUND", responseCodeSuccess, "2500"),
			provision("REFUND", "4001", "1000"), // declined: nothing returned
			provision("REFUND", responseCodeSuccess, "500"),
		},
	})
	if err != nil || led.RefundedMinor != 3000 || led.Reversed {
		t.Fatalf("ledger %+v, err %v; want 3000 refunded, not reversed", led, err)
	}
	if got := rec.logID("providerInquireRequest"); got != 88 {
		t.Errorf("inquiry logged on row %d, want the operation's row 88", got)
	}
}

func TestRefundLedgerSeesAReverse(t *testing.T) {
	led, _, err := ledgerFrom(t, PaycellInquireResponse{
		ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
		ProvisionList:  []PaycellProvisionListItem{provision("SALE", responseCodeSuccess, "50000"), provision("REVERSE", responseCodeSuccess, "50000")},
	})
	if err != nil || !led.Reversed || led.RefundedMinor != 0 {
		t.Fatalf("ledger %+v, err %v; want reversed", led, err)
	}
}

// The guard must not decide on what Paycell has not settled.
func TestRefundLedgerRefusesUnsettledAnswers(t *testing.T) {
	for name, inquire := range map[string]PaycellInquireResponse{
		"refund processing": {
			ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
			ProvisionList:  []PaycellProvisionListItem{provision("REFUND", responseCodeProcessing, "2500")},
		},
		"order not found": {ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound}},
		"bad amount": {
			ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
			ProvisionList:  []PaycellProvisionListItem{provision("REFUND", responseCodeSuccess, "25.00")},
		},
	} {
		if _, _, err := ledgerFrom(t, inquire); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
