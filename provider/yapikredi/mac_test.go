package yapikredi

import "testing"

// Vectors from the POSNET 3D Secure document (2.0.1.6, section 3, "MAC Datasının oluşturulması").
func TestMACMatchesBankVectors(t *testing.T) {
	first := firstHash("10,10,10,10,10,10,10,10", "67005551")
	if first != "c1PPl+2UcdixyhgLYnf4VfJyFGaNQNOwE0uMkci7Uag=" {
		t.Fatalf("firstHash = %s", first)
	}

	mac := requestMAC("YKB_TST_190620093100_024", "175", "TL", "6706598320", first)
	if mac != "J/7/Xprj7F/KDf98luVfIGyUPRQzUCqGwpmvz3KT7oQ=" {
		t.Errorf("requestMAC = %s", mac)
	}
}

func TestResponseMACsPrependTheirField(t *testing.T) {
	first := firstHash("k", "t")
	if got, want := resolveResponseMAC("1", "x", "1", "TL", "m", first), hashB64("1;x;1;TL;m;"+first); got != want {
		t.Errorf("resolveResponseMAC = %s, want %s", got, want)
	}
	if got, want := tranResponseMAC("h", "x", "1", "TL", "m", first), hashB64("h;x;1;TL;m;"+first); got != want {
		t.Errorf("tranResponseMAC = %s, want %s", got, want)
	}
}

func TestMACEqual(t *testing.T) {
	if !macEqual("abc", "abc") {
		t.Error("equal MACs must match")
	}
	if macEqual("abc", "abd") {
		t.Error("different MACs must not match")
	}
	if macEqual("", "") {
		t.Error("an empty received MAC must never match")
	}
}
