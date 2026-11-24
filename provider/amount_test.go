package provider

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMinorUnitsRounds(t *testing.T) {
	for amount, want := range map[float64]int64{
		19.99: 1999, // int(19.99*100) is 1998
		0.29:  29,   // int(0.29*100) is 28
		1.005: 100,  // 1.005 is 1.00499... in float64
		100.5: 10050,
		1500:  150000,
		0:     0,
	} {
		if got := MinorUnits(amount); got != want {
			t.Errorf("MinorUnits(%v) = %d, want %d", amount, got, want)
		}
	}
}

// A line with an integer conversion and a multiplication by 100 is the conversion MinorUnits
// replaces, in any spelling: int(x*100), int64(100*x), int32(x * 100.0).
var (
	integerConversion = regexp.MustCompile(`\bu?int(8|16|32|64)?\(`)
	timesHundred      = regexp.MustCompile(`\*\s*100(\.0*)?\b|\b100(\.0*)?\s*\*`)
)

// Walks the provider package and every provider's source: an amount converted by truncation sends
// one kuruş less.
func TestNoProviderTruncatesAmountsToMinorUnits(t *testing.T) {
	root, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	nested, err := filepath.Glob("*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	walked := 0
	for _, file := range append(root, nested...) {
		if strings.HasSuffix(file, "_test.go") || file == "amount.go" {
			continue
		}
		walked++
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if integerConversion.MatchString(line) && timesHundred.MatchString(line) {
				t.Errorf("%s:%d truncates to minor units; use provider.MinorUnits: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
	if walked < 20 {
		t.Fatalf("walked %d provider source files, the glob no longer finds them", walked)
	}
}
