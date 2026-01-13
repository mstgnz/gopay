package ziraat

import "github.com/mstgnz/gopay/provider"

func init() {
	// Register Ziraat provider with the global registry
	provider.Register("ziraat", NewProvider)
}
