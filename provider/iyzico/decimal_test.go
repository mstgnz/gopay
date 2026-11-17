package iyzico

import (
	"encoding/json"
	"testing"
)

func TestAmountKeepsMoneyExact(t *testing.T) {
	cases := map[string]string{
		`5.20000000`:          "5.2",
		`"5.20000000"`:        "5.2",
		`1500.0`:              "1500",
		`0E-8`:                "0",
		`0.10000000`:          "0.1",
		`450.50`:              "450.5",
		`0.29999999999999999`: "0.29999999999999999", // float64 would make this 0.3
		`12345678901.01`:      "12345678901.01",
		`null`:                "",
		`""`:                  "",
	}
	for in, want := range cases {
		var a amount
		if err := json.Unmarshal([]byte(in), &a); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if string(a) != want {
			t.Errorf("%s: got %q, want %q", in, a, want)
		}
	}
}

func TestAmountRejectsNonNumbers(t *testing.T) {
	for _, in := range []string{`"abc"`, `true`, `"1.2.3"`} {
		var a amount
		if err := json.Unmarshal([]byte(in), &a); err == nil {
			t.Errorf("%s: want an error, got %q", in, a)
		}
	}
}

func TestAmountAbsentStaysEmpty(t *testing.T) {
	var v struct {
		Price amount `json:"price"`
	}
	if err := json.Unmarshal([]byte(`{}`), &v); err != nil || v.Price != "" {
		t.Fatalf("got %q, %v", v.Price, err)
	}
}

func TestFlexStringReadsStringsAndNumbers(t *testing.T) {
	cases := map[string]string{`"10051"`: "10051", `50000`: "50000", `28157797`: "28157797", `null`: ""}
	for in, want := range cases {
		var f flexString
		if err := json.Unmarshal([]byte(in), &f); err != nil || string(f) != want {
			t.Errorf("%s: got %q, %v", in, f, err)
		}
	}
	var f flexString
	if err := json.Unmarshal([]byte(`{}`), &f); err == nil {
		t.Error("an object must not read as a flexString")
	}
}

func TestFlexIntKeepsAbsentApartFromZero(t *testing.T) {
	var v struct {
		Fraud flexInt `json:"fraudStatus"`
	}
	if err := json.Unmarshal([]byte(`{}`), &v); err != nil || v.Fraud.ptr() != nil {
		t.Fatalf("absent fraudStatus must stay unset, got %+v %v", v.Fraud, err)
	}
	for in, want := range map[string]int{`{"fraudStatus":0}`: 0, `{"fraudStatus":-1}`: -1, `{"fraudStatus":"1"}`: 1} {
		v.Fraud = flexInt{}
		if err := json.Unmarshal([]byte(in), &v); err != nil || v.Fraud.ptr() == nil || *v.Fraud.ptr() != want {
			t.Errorf("%s: got %+v, %v", in, v.Fraud, err)
		}
	}
	if err := json.Unmarshal([]byte(`{"fraudStatus":1.5}`), &v); err == nil {
		t.Error("a fraction must not read as an integer status")
	}
	if err := json.Unmarshal([]byte(`{"fraudStatus":null}`), &v); err != nil || v.Fraud.ptr() != nil {
		t.Errorf("null must read as unset, got %+v %v", v.Fraud, err)
	}
}
