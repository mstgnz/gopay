package akbank

import "github.com/mstgnz/gopay/provider"

func init() {
	// Register Akbank provider with the global registry
	provider.Register("akbank", NewProvider)
}
