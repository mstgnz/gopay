package yapikredi

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/provider"
)

// Create3DPayment encrypts the payment at the bank and returns the form that takes the payer to
// the bank's 3D page. Nothing is charged until Complete3DPayment finances it.
func (p *YapiKrediProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	in, err := p.paymentInput(request)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: invalid 3D payment request: %w", err)
	}
	if strings.TrimSpace(request.CallbackURL) == "" {
		return nil, errors.New("yapikredi: invalid 3D payment request: callback URL is required")
	}
	xid, err := p.newID(xidRandomLen)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: %w", err)
	}
	p.record("oosRequest", map[string]any{"xid": xid, "amount": in.amount, "currency": in.currency, "installment": in.installment, "tranType": "Sale"})

	resp, err := p.call(ctx, posnetRequest{OOSRequestData: &oosRequestData{
		PosnetID: p.posnetID, XID: xid, Amount: in.amount, CurrencyCode: in.currency,
		Installment: in.installment, TranType: "Sale", CardHolderName: in.cardHolder,
		CCNo: in.ccno, ExpDate: in.expDate, CVC: in.cvc,
	}})
	if err != nil {
		// Nothing is charged before the bank page; the start simply failed.
		return nil, fmt.Errorf("yapikredi: 3D start failed: %w", err)
	}
	p.record("oosResponse", resultFields(resp))

	out := &provider.PaymentResponse{
		PaymentID:  xid,
		OrderID:    xid,
		Amount:     request.Amount,
		Currency:   request.Currency,
		SystemTime: timePtr(p.now()),
	}
	if resp.Approved != approvedYes || resp.OOSData == nil || resp.OOSData.Data1 == "" || resp.OOSData.Sign == "" {
		out.Status, out.ErrorCode = provider.StatusFailed, resp.RespCode
		out.Message = failureText(resp.RespText, "3D payment could not be started")
		out.ProviderResponse = resultFields(resp)
		return out, nil
	}

	// The state is stored only for a start the bank accepted, so a rejected start leaves no open
	// callback and its status reads failed at once.
	callbackURL, err := p.storeCallback(ctx, p.gopayBaseURL, provider.CallbackState{
		PaymentID:        xid,
		TenantID:         request.TenantID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		LogID:            request.LogID,
		Provider:         providerName,
		Environment:      request.Environment,
		Timestamp:        p.now(),
		OriginalCallback: request.CallbackURL,
		ClientIP:         request.ClientIP,
		Installment:      request.InstallmentCount,
		SessionID:        request.SessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("yapikredi: failed to store callback state: %w", err)
	}
	if len(callbackURL) > maxReturnURLLength {
		return nil, fmt.Errorf("yapikredi: callback URL is %d characters, Posnet accepts %d", len(callbackURL), maxReturnURLLength)
	}

	out.Success, out.Status, out.Message = true, provider.StatusPending, "Redirect the payer to the bank's 3D page"
	out.HTML = p.threeDForm(resp.OOSData, callbackURL)
	return out, nil
}

// threeDForm is the auto-submitting form to the bank's 3D page. posnetData2 is the bank-encrypted
// card; the sanitizer blanks it in the logged copy.
func (p *YapiKrediProvider) threeDForm(data *oosRequestResult, callbackURL string) string {
	fields := [][2]string{
		{"mid", p.merchantID},
		{"posnetID", p.posnetID},
		{"posnetData", data.Data1},
		{"posnetData2", data.Data2},
		{"digest", data.Sign},
		{"vftCode", ""},
		{"merchantReturnURL", callbackURL},
		{"lang", "tr"},
		{"url", ""},
		{"openANewWindow", "0"},
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>3D Secure</title></head><body>`)
	fmt.Fprintf(&b, `<form id="posnet3d" name="posnet3d" method="post" action="%s">`, html.EscapeString(p.threeDURL))
	for _, f := range fields {
		fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, html.EscapeString(f[0]), html.EscapeString(f[1]))
	}
	b.WriteString(`<noscript><button type="submit">Continue</button></noscript></form>`)
	b.WriteString(`<script>document.getElementById('posnet3d').submit();</script></body></html>`)
	return b.String()
}

// Complete3DPayment finances a 3D payment the payer authenticated. The browser form is not trusted:
// the bank resolves its packets, and every check runs on that server-to-server answer.
func (p *YapiKrediProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID
	xid := callbackState.PaymentID
	out := &provider.PaymentResponse{
		PaymentID:   xid,
		OrderID:     xid,
		Amount:      callbackState.Amount,
		Currency:    callbackState.Currency,
		RedirectURL: callbackState.OriginalCallback,
		SystemTime:  timePtr(p.now()),
	}
	fail := func(code, message string) (*provider.PaymentResponse, error) {
		out.Status, out.ErrorCode, out.Message = provider.StatusFailed, code, message
		return out, nil
	}

	merchantPacket, bankPacket, sign := formValue(data, "MerchantPacket"), formValue(data, "BankPacket"), formValue(data, "Sign")
	if merchantPacket == "" || bankPacket == "" || sign == "" {
		return fail(errCodeMissing3DData, "3D result packets are missing")
	}
	amount, err := kurus(callbackState.Amount)
	if err != nil {
		return fail(errCodeDataMismatch, "3D payment amount is invalid")
	}
	first := firstHash(p.encKey, p.terminalID)
	mac := requestMAC(xid, amount, posnetCurrencyTL, p.merchantID, first)

	// Before oosTranData nothing is charged and the callback state is spent, so every problem
	// up to that call is a final failure.
	resolved, err := p.call(ctx, posnetRequest{OOSResolve: &oosResolveMerchantData{
		BankData: bankPacket, MerchantData: merchantPacket, Sign: sign, MAC: mac,
	}})
	if err != nil {
		logger.Warn("Yapı Kredi 3D resolve got no answer", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": xid, "error": err.Error()},
		})
		return fail(errCodeResolveFailed, "3D result could not be resolved")
	}
	p.record("resolveResponse", resolveFields(resolved))

	if resolved.Approved != approvedYes || resolved.Resolve == nil {
		return fail(resolved.RespCode, failureText(resolved.RespText, "3D result was rejected"))
	}
	r := resolved.Resolve
	if r.MDStatus != mdStatusOK {
		return fail(errCodeAuthFailed, failureText(r.MDErrorMessage, "3D authentication failed (mdStatus "+r.MDStatus+")"))
	}
	// Compared before the MAC only so the error names the cause: the bank's MAC covers its own
	// values, so a mismatch here would also fail the MAC.
	if r.XID != xid || r.Amount != amount || !strings.EqualFold(r.Currency, posnetCurrencyTL) {
		logger.Warn("Yapı Kredi 3D resolve differs from the payment", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": xid, "xid": r.XID, "amount": r.Amount, "currency": r.Currency},
		})
		return fail(errCodeDataMismatch, "3D result does not match the payment")
	}
	if !macEqual(r.MAC, resolveResponseMAC(r.MDStatus, xid, amount, posnetCurrencyTL, p.merchantID, first)) {
		logger.Warn("Yapı Kredi 3D resolve MAC does not match", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": xid},
		})
		return fail(errCodeMACMismatch, "3D result could not be verified")
	}

	tran, err := p.call(ctx, posnetRequest{OOSTran: &oosTranData{BankData: bankPacket, WPAmount: "0", MAC: mac}})
	if err != nil {
		// The finance request may have reached the bank; the status query settles it.
		logger.Warn("Yapı Kredi 3D finance got no answer", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": xid, "error": err.Error()},
		})
		out.Status, out.ErrorCode, out.Message = provider.StatusPending, errCodeUnverified, "3D payment result could not be verified"
		return out, nil
	}
	p.record("tranResponse", resultFields(tran))
	out.ProviderResponse = resultFields(tran)

	if tran.Approved != approvedYes && tran.Approved != approvedAlready {
		return fail(tran.RespCode, failureText(tran.RespText, "3D payment failed"))
	}
	if !macEqual(tran.MAC, tranResponseMAC(tran.HostLogKey, xid, amount, posnetCurrencyTL, p.merchantID, first)) {
		logger.Warn("Yapı Kredi 3D finance MAC does not match", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": xid, "approved": tran.Approved},
		})
		out.Status, out.ErrorCode, out.Message = provider.StatusPending, errCodeUnverified, "3D payment result could not be verified"
		return out, nil
	}

	out.Success, out.Status, out.Message = true, provider.StatusSuccessful, "3D payment completed successfully"
	out.TransactionID = tran.HostLogKey
	return out, nil
}

// resolveFields are the resolve answer values safe to log; MACs and packets are left out.
func resolveFields(r *posnetResponse) map[string]any {
	fields := map[string]any{"approved": r.Approved, "respCode": r.RespCode, "respText": r.RespText}
	if r.Resolve != nil {
		fields["xid"] = r.Resolve.XID
		fields["amount"] = r.Resolve.Amount
		fields["currency"] = r.Resolve.Currency
		fields["mdStatus"] = r.Resolve.MDStatus
		fields["mdErrorMessage"] = r.Resolve.MDErrorMessage
	}
	return fields
}

// formValue reads a callback field whatever case the bank used for its name.
func formValue(data map[string]string, name string) string {
	if v, ok := data[name]; ok {
		return strings.TrimSpace(v)
	}
	for k, v := range data {
		if strings.EqualFold(k, name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
