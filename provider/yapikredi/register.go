package yapikredi

import "github.com/mstgnz/gopay/provider"

// Register the Yapı Kredi provider with the gateway registry
func init() {
	provider.Register(providerName, NewProvider)
}
