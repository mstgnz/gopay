package yapikredi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// hashB64 is Posnet's HASH: Base64(SHA-256(UTF-8 bytes)).
func hashB64(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// firstHash is HASH(encKey;terminalID), the merchant secret part of every MAC.
func firstHash(encKey, terminalID string) string {
	return hashB64(encKey + ";" + terminalID)
}

// requestMAC is the MAC GoPay sends with oosResolveMerchantData and oosTranData.
func requestMAC(xid, amount, currency, merchantID, first string) string {
	return hashB64(strings.Join([]string{xid, amount, currency, merchantID, first}, ";"))
}

// resolveResponseMAC is the MAC the bank puts in oosResolveMerchantDataResponse.
func resolveResponseMAC(mdStatus, xid, amount, currency, merchantID, first string) string {
	return hashB64(strings.Join([]string{mdStatus, xid, amount, currency, merchantID, first}, ";"))
}

// tranResponseMAC is the MAC the bank puts in the oosTranData response.
func tranResponseMAC(hostLogKey, xid, amount, currency, merchantID, first string) string {
	return hashB64(strings.Join([]string{hostLogKey, xid, amount, currency, merchantID, first}, ";"))
}

// macEqual compares MACs in constant time; an empty received MAC never matches.
func macEqual(received, expected string) bool {
	return received != "" && hmac.Equal([]byte(received), []byte(expected))
}
