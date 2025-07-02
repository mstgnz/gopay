package paycell

import (
	"github.com/mstgnz/gopay/provider"
)

func init() {
	provider.Register("paycell", NewProvider)
}
