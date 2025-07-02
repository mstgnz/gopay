package ozanpay

import "github.com/mstgnz/gopay/provider"

// Register OzanPay provider with the gateway registry
func init() {
	provider.Register("ozanpay", NewProvider)
}
