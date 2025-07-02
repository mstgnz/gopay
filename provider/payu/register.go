package payu

import "github.com/mstgnz/gopay/provider"

func init() {
	provider.Register("payu", NewProvider)
}
