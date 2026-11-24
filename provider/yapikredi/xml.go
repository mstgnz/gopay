package yapikredi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// xmlDeclaration heads every request. The bank documents UTF-8 as supported; its samples declare
// ISO-8859-9, which would garble a Turkish cardholder name sent as UTF-8 bytes.
const xmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>`

// posnetRequest is the single request envelope; exactly one operation block is set.
type posnetRequest struct {
	XMLName          xml.Name                `xml:"posnetRequest"`
	MID              string                  `xml:"mid"`
	TID              string                  `xml:"tid"`
	TranDateRequired string                  `xml:"tranDateRequired,omitempty"`
	OOSRequestData   *oosRequestData         `xml:"oosRequestData,omitempty"`
	OOSResolve       *oosResolveMerchantData `xml:"oosResolveMerchantData,omitempty"`
	OOSTran          *oosTranData            `xml:"oosTranData,omitempty"`
	Sale             *saleData               `xml:"sale,omitempty"`
	Reverse          *reverseData            `xml:"reverse,omitempty"`
	Return           *returnData             `xml:"return,omitempty"`
	Agreement        *agreementData          `xml:"agreement,omitempty"`
}

type oosRequestData struct {
	PosnetID       string `xml:"posnetid"`
	XID            string `xml:"XID"`
	Amount         string `xml:"amount"`
	CurrencyCode   string `xml:"currencyCode"`
	Installment    string `xml:"installment"`
	TranType       string `xml:"tranType"`
	CardHolderName string `xml:"cardHolderName"`
	CCNo           string `xml:"ccno"`
	ExpDate        string `xml:"expDate"`
	CVC            string `xml:"cvc"`
}

type oosResolveMerchantData struct {
	BankData     string `xml:"bankData"`
	MerchantData string `xml:"merchantData"`
	Sign         string `xml:"sign"`
	MAC          string `xml:"mac"`
}

type oosTranData struct {
	BankData string `xml:"bankData"`
	WPAmount string `xml:"wpAmount"`
	MAC      string `xml:"mac"`
}

type saleData struct {
	Amount       string `xml:"amount"`
	CCNo         string `xml:"ccno"`
	CurrencyCode string `xml:"currencyCode"`
	CVC          string `xml:"cvc"`
	ExpDate      string `xml:"expDate"`
	OrderID      string `xml:"orderID"`
	Installment  string `xml:"installment"`
}

type reverseData struct {
	Transaction string `xml:"transaction"`
	HostLogKey  string `xml:"hostLogKey"`
}

type returnData struct {
	Amount       string `xml:"amount"`
	CurrencyCode string `xml:"currencyCode"`
	HostLogKey   string `xml:"hostLogKey"`
}

type agreementData struct {
	OrderID string `xml:"orderID"`
}

// posnetResponse covers every answer GoPay reads. Sale-type answers spell the key "hostlogkey",
// agreement rows "hostLogKey"; XML is case-sensitive, so both are mapped.
type posnetResponse struct {
	XMLName    xml.Name          `xml:"posnetResponse"`
	Approved   string            `xml:"approved"`
	RespCode   string            `xml:"respCode"`
	RespText   string            `xml:"respText"`
	MAC        string            `xml:"mac"`
	HostLogKey string            `xml:"hostlogkey"`
	AuthCode   string            `xml:"authCode"`
	OOSData    *oosRequestResult `xml:"oosRequestDataResponse"`
	Resolve    *resolveResult    `xml:"oosResolveMerchantDataResponse"`
	Txns       []agreementTxn    `xml:"transactions>transaction"`
}

type oosRequestResult struct {
	Data1 string `xml:"data1"`
	Data2 string `xml:"data2"`
	Sign  string `xml:"sign"`
}

type resolveResult struct {
	XID            string `xml:"xid"`
	Amount         string `xml:"amount"`
	Currency       string `xml:"currency"`
	Installment    string `xml:"installment"`
	TxStatus       string `xml:"txStatus"`
	MDStatus       string `xml:"mdStatus"`
	MDErrorMessage string `xml:"mdErrorMessage"`
	MAC            string `xml:"mac"`
}

// agreementTxn is one agreement row. The bank's sample spells the status txnStatus, its field table
// txstatus; every spelling is read so a renamed element cannot turn into an empty status.
type agreementTxn struct {
	OrderID       string `xml:"orderID"`
	Amount        string `xml:"amount"`
	CurrencyCode  string `xml:"currencyCode"`
	AuthCode      string `xml:"authCode"`
	TranDate      string `xml:"tranDate"`
	State         string `xml:"state"`
	TxnStatus     string `xml:"txnStatus"`
	TxStatus      string `xml:"txStatus"`
	TxStatusLower string `xml:"txstatus"`
	HostLogKey    string `xml:"hostLogKey"`
}

// status is the row's status under whichever spelling the bank used.
func (t agreementTxn) status() string {
	for _, s := range []string{t.TxnStatus, t.TxStatus, t.TxStatusLower} {
		if s != "" {
			return s
		}
	}
	return ""
}

// schemaErrorMarkers identify the bank's XML schema rejections (XNIException, cvc-* constraint
// codes), whose text echoes the rejected value; that value can be a card field.
var schemaErrorMarkers = []string{"xniexception", "cvc-"}

// cardLikeDigits is a PAN-length digit run.
var cardLikeDigits = regexp.MustCompile(`\d{12,19}`)

// safeBankText keeps bank messages free of card data before they are logged or returned.
func safeBankText(s string) string {
	lower := strings.ToLower(s)
	for _, m := range schemaErrorMarkers {
		if strings.Contains(lower, m) {
			return "Request rejected by the bank's schema check"
		}
	}
	return cardLikeDigits.ReplaceAllString(s, "****")
}

// encodeRequest renders the request document. encoding/xml escapes every value.
func encodeRequest(req posnetRequest) (string, error) {
	body, err := xml.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal posnet request: %w", err)
	}
	return xmlDeclaration + string(body), nil
}

// decodeResponse reads a bank answer. Answers declare iso-8859-9; the agreement sample declares
// utf-16. Real UTF-16 bytes (a BOM or NUL bytes) are transcoded first; a declaration that no
// longer matches the bytes is then read as UTF-8.
func decodeResponse(body []byte) (*posnetResponse, error) {
	data, utf16Bytes, err := fromUTF16(body)
	if err != nil {
		return nil, err
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "iso-8859-9", "iso8859-9":
			if utf16Bytes {
				return input, nil
			}
			return charmap.ISO8859_9.NewDecoder().Reader(input), nil
		case "utf-16", "utf16", "utf-8", "utf8", "":
			return input, nil
		default:
			return nil, fmt.Errorf("unsupported response charset %q", label)
		}
	}

	var resp posnetResponse
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("unreadable posnet response: %w", err)
	}
	resp.trim()
	resp.RespText = safeBankText(resp.RespText)
	if resp.Resolve != nil {
		resp.Resolve.MDErrorMessage = safeBankText(resp.Resolve.MDErrorMessage)
	}
	return &resp, nil
}

// fromUTF16 transcodes a UTF-16 body to UTF-8 and reports whether it did.
func fromUTF16(body []byte) ([]byte, bool, error) {
	var endian unicode.Endianness
	switch {
	case bytes.HasPrefix(body, []byte{0xFE, 0xFF}):
		endian = unicode.BigEndian
	case bytes.HasPrefix(body, []byte{0xFF, 0xFE}):
		endian = unicode.LittleEndian
	case len(body) > 1 && body[0] == 0:
		endian = unicode.BigEndian
	case len(body) > 1 && body[1] == 0:
		endian = unicode.LittleEndian
	default:
		return body, false, nil
	}
	out, _, err := transform.Bytes(unicode.UTF16(endian, unicode.UseBOM).NewDecoder(), body)
	if err != nil {
		return nil, false, fmt.Errorf("unreadable utf-16 posnet response: %w", err)
	}
	return out, true, nil
}

// trim drops the whitespace the bank wraps long values in.
func (r *posnetResponse) trim() {
	for _, s := range []*string{&r.Approved, &r.RespCode, &r.RespText, &r.MAC, &r.HostLogKey, &r.AuthCode} {
		*s = strings.TrimSpace(*s)
	}
	if r.OOSData != nil {
		for _, s := range []*string{&r.OOSData.Data1, &r.OOSData.Data2, &r.OOSData.Sign} {
			*s = strings.TrimSpace(*s)
		}
	}
	if r.Resolve != nil {
		x := r.Resolve
		for _, s := range []*string{&x.XID, &x.Amount, &x.Currency, &x.Installment, &x.TxStatus, &x.MDStatus, &x.MDErrorMessage, &x.MAC} {
			*s = strings.TrimSpace(*s)
		}
	}
	for i := range r.Txns {
		t := &r.Txns[i]
		for _, s := range []*string{&t.OrderID, &t.Amount, &t.CurrencyCode, &t.AuthCode, &t.TranDate, &t.State, &t.TxnStatus, &t.TxStatus, &t.TxStatusLower, &t.HostLogKey} {
			*s = strings.TrimSpace(*s)
		}
	}
}
