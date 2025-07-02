package nkolay

import "github.com/mstgnz/gopay/provider"

// Register Nkolay provider with the gateway registry
func init() {
	provider.Register("nkolay", NewProvider)
}
