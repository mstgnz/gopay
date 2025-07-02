package iyzico

import "github.com/mstgnz/gopay/provider"

// Register Iyzico provider with the gateway registry
func init() {
	provider.Register("iyzico", NewProvider)
}
