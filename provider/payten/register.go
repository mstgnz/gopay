package payten

import "github.com/mstgnz/gopay/provider"

func init() {
	// Register Payten provider with the global registry
	provider.Register("payten", NewProvider)
}
