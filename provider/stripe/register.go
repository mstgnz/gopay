package stripe

import "github.com/mstgnz/gopay/provider"

// Register Stripe provider with the gateway registry
func init() {
	provider.Register("stripe", NewProvider)
}
