package yapikredi

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func TestEncodeRequestEscapesAndOmitsEmptyBlocks(t *testing.T) {
	got, err := encodeRequest(posnetRequest{
		MID: "6700000001", TID: "67000001",
		OOSRequestData: &oosRequestData{XID: "X", CardHolderName: `A&B <C>`},
	})
	if err != nil {
		t.Fatalf("encodeRequest: %v", err)
	}
	if !strings.HasPrefix(got, xmlDeclaration+"<posnetRequest><mid>6700000001</mid><tid>67000001</tid><oosRequestData>") {
		t.Errorf("unexpected document start: %s", got)
	}
	if !strings.Contains(got, "<cardHolderName>A&amp;B &lt;C&gt;</cardHolderName>") {
		t.Errorf("value not escaped: %s", got)
	}
	for _, absent := range []string{"<sale>", "<reverse>", "<return>", "<agreement>", "<tranDateRequired>"} {
		if strings.Contains(got, absent) {
			t.Errorf("unset block %s rendered: %s", absent, got)
		}
	}
}

func TestDecodeResponseISO88599(t *testing.T) {
	text := "<?xml version='1.0' encoding='iso-8859-9'?><posnetResponse><approved>0</approved>" +
		"<respCode>0005</respCode><respText>Kart bilgileri hatalı, ŞĞİ</respText></posnetResponse>"
	body, err := charmap.ISO8859_9.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	resp, err := decodeResponse(body)
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if resp.Approved != "0" || resp.RespCode != "0005" || resp.RespText != "Kart bilgileri hatalı, ŞĞİ" {
		t.Errorf("decoded %+v", resp)
	}
}

func TestDecodeResponseUTF16Agreement(t *testing.T) {
	text := `<?xml version="1.0" encoding="utf-16"?><posnetResponse><approved>1</approved><transactions>` +
		`<transaction><orderID> TDS_GP2611241200000ABCDEF </orderID><state>Sale</state><txnStatus>1</txnStatus>` +
		`<hostLogKey>021450428990000191</hostLogKey></transaction></transactions></posnetResponse>`
	for _, endian := range []unicode.Endianness{unicode.LittleEndian, unicode.BigEndian} {
		for _, bom := range []unicode.BOMPolicy{unicode.UseBOM, unicode.IgnoreBOM} {
			body, err := unicode.UTF16(endian, bom).NewEncoder().Bytes([]byte(text))
			if err != nil {
				t.Fatalf("encode fixture: %v", err)
			}
			resp, err := decodeResponse(body)
			if err != nil {
				t.Fatalf("decodeResponse (endian %v, bom %v): %v", endian, bom, err)
			}
			if len(resp.Txns) != 1 || resp.Txns[0].OrderID != "TDS_GP2611241200000ABCDEF" || resp.Txns[0].HostLogKey != "021450428990000191" {
				t.Errorf("decoded %+v", resp.Txns)
			}
		}
	}
}

// The agreement sample declares utf-16 but may arrive as plain bytes.
func TestDecodeResponseUTF16DeclarationOnUTF8Bytes(t *testing.T) {
	resp, err := decodeResponse([]byte(`<?xml version="1.0" encoding="utf-16"?><posnetResponse><approved>1</approved></posnetResponse>`))
	if err != nil || resp.Approved != "1" {
		t.Fatalf("got %+v, %v", resp, err)
	}
}

func TestDecodeResponseTrimsWrappedValues(t *testing.T) {
	resp, err := decodeResponse([]byte("<posnetResponse><approved>1</approved><oosRequestDataResponse><data1>AB\nCD </data1>" +
		"<data2> EF</data2><sign>\n12\n</sign></oosRequestDataResponse><oosResolveMerchantDataResponse><xid> X </xid>" +
		"<mdStatus> 1 </mdStatus></oosResolveMerchantDataResponse></posnetResponse>"))
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if resp.OOSData.Data2 != "EF" || resp.OOSData.Sign != "12" || resp.Resolve.XID != "X" || resp.Resolve.MDStatus != "1" {
		t.Errorf("values not trimmed: %+v %+v", resp.OOSData, resp.Resolve)
	}
}

func TestDecodeResponseRejectsUnknownCharsetAndGarbage(t *testing.T) {
	if _, err := decodeResponse([]byte(`<?xml version="1.0" encoding="koi8-r"?><posnetResponse/>`)); err == nil {
		t.Error("unknown charset accepted")
	}
	if _, err := decodeResponse([]byte(`<html>Service Unavailable`)); err == nil {
		t.Error("non-XML body accepted")
	}
	if _, err := decodeResponse([]byte(`<html><approved>1</approved></html>`)); err == nil {
		t.Error("a document that is not a posnetResponse was accepted")
	}
}
