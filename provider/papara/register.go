package papara

import "github.com/mstgnz/gopay/provider"

// Register Papara provider with the gateway registry
func init() {
	provider.Register("papara", NewProvider)
}
