package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mstgnz/gopay/infra/config"
)

var letterRunes = []rune("0987654321abcçdefgğhıijklmnoöpqrsştuüvwxyzABCÇDEFGĞHIİJKLMNOÖPQRSTUÜVWXYZ-_!?+&%=*")

// GenerateToken token generate
func GenerateToken(userId int) (string, error) {
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().AddDate(0, 0, 1)),
		Issuer:    strconv.Itoa(userId),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(config.App().SecretKey))
	if err != nil {
		return "", err
	}
	return t, nil
}

// ValidateToken token validate
func ValidateToken(token string) (*jwt.Token, error) {
	return jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(config.App().SecretKey), nil
	})
}

func GetUserIDByToken(token string) (string, error) {
	valid, err := ValidateToken(token)
	if err != nil {
		return "", err
	}
	claims := valid.Claims.(jwt.MapClaims)
	id := fmt.Sprintf("%v", claims["iss"])
	return id, nil
}

func RandomString(length int) string {
	s, r := make([]rune, length), []rune(letterRunes)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}

func RandomHex(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return RandomString(length)
	}
	return hex.EncodeToString(bytes)
}
