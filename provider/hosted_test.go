package provider

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func validCheckout() HostedCheckoutRequest {
	return HostedCheckoutRequest{
		BasketID: "B-1", Price: "450.50", Currency: "TRY", CallbackURL: "https://tenant.example/odeme",
		Buyer: CheckoutBuyer{IP: "85.34.78.112"},
		Items: []CheckoutItem{
			{ID: "1", ItemType: ItemTypeVirtual, Price: "400.25", SubMerchantKey: "smk", SubMerchantPrice: "360.20"},
			{ID: "2", ItemType: ItemTypeVirtual, Price: "50.25"},
		},
	}
}

func TestValidateHostedCheckoutAcceptsAnExactBasket(t *testing.T) {
	if err := ValidateHostedCheckout(validCheckout(), "production"); err != nil {
		t.Fatalf("valid basket refused: %v", err)
	}
	r := validCheckout()
	r.CallbackURL = "http://localhost:3000/odeme"
	if err := ValidateHostedCheckout(r, "sandbox"); err != nil {
		t.Fatalf("http callback must be allowed in sandbox: %v", err)
	}
}

func TestValidateHostedCheckoutRefusals(t *testing.T) {
	cases := map[string]struct {
		edit func(*HostedCheckoutRequest)
		env  string
		want string
	}{
		"price not the item sum":   {func(r *HostedCheckoutRequest) { r.Price = "450.49" }, "production", "sum of the item prices"},
		"float-looking amount":     {func(r *HostedCheckoutRequest) { r.Price = "4.505e2" }, "production", "not a decimal amount"},
		"three fraction digits":    {func(r *HostedCheckoutRequest) { r.Items[1].Price = "50.250" }, "production", "at most 2 fraction digits"},
		"negative price":           {func(r *HostedCheckoutRequest) { r.Price = "-1" }, "production", "not a decimal amount"},
		"zero item":                {func(r *HostedCheckoutRequest) { r.Items[1].Price = "0" }, "production", "greater than 0"},
		"leading zero":             {func(r *HostedCheckoutRequest) { r.Items[1].Price = "050.25" }, "production", "not a decimal amount"},
		"share above item":         {func(r *HostedCheckoutRequest) { r.Items[0].SubMerchantPrice = "400.26" }, "production", "must not exceed"},
		"zero share":               {func(r *HostedCheckoutRequest) { r.Items[0].SubMerchantPrice = "0.00" }, "production", "greater than 0"},
		"key without share":        {func(r *HostedCheckoutRequest) { r.Items[0].SubMerchantPrice = "" }, "production", "come together"},
		"share without key":        {func(r *HostedCheckoutRequest) { r.Items[0].SubMerchantKey = "" }, "production", "come together"},
		"unknown item type":        {func(r *HostedCheckoutRequest) { r.Items[0].ItemType = "SERVICE" }, "production", "itemType"},
		"physical without address": {func(r *HostedCheckoutRequest) { r.Items[0].ItemType = ItemTypePhysical }, "production", "shippingAddress"},
		"no items":                 {func(r *HostedCheckoutRequest) { r.Items = nil }, "production", "items must not be empty"},
		"other currency":           {func(r *HostedCheckoutRequest) { r.Currency = "USD" }, "production", "currency"},
		"http in production":       {func(r *HostedCheckoutRequest) { r.CallbackURL = "http://tenant.example" }, "production", "https"},
		"javascript callback":      {func(r *HostedCheckoutRequest) { r.CallbackURL = "javascript:alert(1)" }, "sandbox", "absolute URL"},
		"relative callback":        {func(r *HostedCheckoutRequest) { r.CallbackURL = "/odeme" }, "sandbox", "absolute URL"},
		"buyer ip":                 {func(r *HostedCheckoutRequest) { r.Buyer.IP = "127.0.0.1;x" }, "production", "buyer.ip"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := validCheckout()
			tc.edit(&r)
			err := ValidateHostedCheckout(r, tc.env)
			if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want invalid mentioning %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateHostedCheckoutPhysicalWithAddress(t *testing.T) {
	r := validCheckout()
	r.Items[0].ItemType = ItemTypePhysical
	r.ShippingAddress = &CheckoutAddress{ContactName: "Ada", Address: "A", City: "Istanbul"}
	if err := ValidateHostedCheckout(r, "production"); err != nil {
		t.Fatalf("physical basket with an address refused: %v", err)
	}
}

func TestValidateHostedCancelAndRefund(t *testing.T) {
	if err := ValidateHostedCancel(HostedCancelRequest{PaymentID: "1", IP: "1.2.3.4", Reason: "BUYER_REQUEST", Description: "x"}); err != nil {
		t.Errorf("valid cancel refused: %v", err)
	}
	for name, r := range map[string]HostedCancelRequest{
		"no payment":              {IP: "1.2.3.4"},
		"bad ip":                  {PaymentID: "1", IP: "nope"},
		"description sans reason": {PaymentID: "1", IP: "1.2.3.4", Description: "x"},
		"unknown reason":          {PaymentID: "1", IP: "1.2.3.4", Reason: "buyer_request"},
	} {
		if err := ValidateHostedCancel(r); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("cancel %s: want invalid, got %v", name, err)
		}
	}

	good := HostedRefundRequest{PaymentTransactionID: "7", Price: "10.5", Currency: "TRY", IP: "2001:db8::1"}
	if err := ValidateHostedRefund(good); err != nil {
		t.Errorf("valid refund refused: %v", err)
	}
	for name, edit := range map[string]func(*HostedRefundRequest){
		"no transaction": func(r *HostedRefundRequest) { r.PaymentTransactionID = " " },
		"zero price":     func(r *HostedRefundRequest) { r.Price = "0" },
		"bad price":      func(r *HostedRefundRequest) { r.Price = "10,5" },
		"currency":       func(r *HostedRefundRequest) { r.Currency = "EUR" },
		"bad ip":         func(r *HostedRefundRequest) { r.IP = "" },
		"reason":         func(r *HostedRefundRequest) { r.Reason = "X" },
	} {
		r := good
		edit(&r)
		if err := ValidateHostedRefund(r); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("refund %s: want invalid, got %v", name, err)
		}
	}
}

func TestParseAmountIsExact(t *testing.T) {
	a, err := ParseAmount("0.10")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ParseAmount("0.20")
	c, _ := ParseAmount("0.30")
	if new(big.Rat).Add(a, b).Cmp(c) != 0 {
		t.Error("0.10 + 0.20 must equal 0.30 exactly")
	}
	if _, err := ParseAmount("999999999999.99"); err != nil {
		t.Errorf("12 integer digits must be accepted: %v", err)
	}
	if _, err := ParseAmount("1000000000000"); err == nil {
		t.Error("13 integer digits must be refused")
	}
}

func TestProviderRefusalError(t *testing.T) {
	err := error(&ProviderRefusal{Code: "10223", Message: "End of day required for the POS."})
	if !strings.Contains(err.Error(), "10223") {
		t.Errorf("refusal text = %q", err.Error())
	}
}
