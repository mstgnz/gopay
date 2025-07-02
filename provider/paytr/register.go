package paytr

import "github.com/mstgnz/gopay/provider"

// Register PayTR provider with the gateway registry
func init() {
	provider.Register("paytr", NewProvider)
}
