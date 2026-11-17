package iyzico

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// maxFractionDigits bounds normalisation; iyzico answers at scale 8.
const maxFractionDigits = 20

// amount is an iyzico answer amount kept exact. iyzico writes them as JSON numbers at scale 8
// ("5.20000000", "0E-8"); float64 would round money, so the literal is parsed as a rational and
// written back with trailing zeros stripped ("5.2", "0"), the form iyzico signs. A JSON string is
// accepted too; null and absent stay "".
type amount string

func (a *amount) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*a = ""
		return nil
	}
	s := strings.Trim(string(b), `"`)
	if s == "" {
		*a = ""
		return nil
	}
	n, err := normaliseAmount(s)
	if err != nil {
		return err
	}
	*a = amount(n)
	return nil
}

// normaliseAmount writes a decimal literal exactly, without trailing fraction zeros.
func normaliseAmount(s string) (string, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", fmt.Errorf("iyzico: %q is not a decimal amount", s)
	}
	for digits := 0; digits <= maxFractionDigits; digits++ {
		text := r.FloatString(digits)
		if back, _ := new(big.Rat).SetString(text); back.Cmp(r) == 0 {
			if text == "-0" {
				text = "0"
			}
			return text, nil
		}
	}
	return "", fmt.Errorf("iyzico: %q has more than %d fraction digits", s, maxFractionDigits)
}

// flexString reads a field iyzico sends as a string or a number ("errorCode": "10051" or 50000,
// "paymentId": "123" or 123).
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("iyzico: %s is neither a string nor a number", b)
	}
	*f = flexString(n.String())
	return nil
}

// flexInt reads an integer status iyzico may send as a number or a string; absent stays nil.
type flexInt struct {
	set   bool
	value int
}

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*f = flexInt{}
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(bytes.Trim(b, `"`), &n); err != nil {
		return fmt.Errorf("iyzico: %s is not an integer", b)
	}
	v, err := n.Int64()
	if err != nil {
		return fmt.Errorf("iyzico: %s is not an integer", b)
	}
	*f = flexInt{set: true, value: int(v)}
	return nil
}

func (f flexInt) ptr() *int {
	if !f.set {
		return nil
	}
	v := f.value
	return &v
}
