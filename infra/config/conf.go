package config

import (
	"github.com/go-playground/validator/v10"
)

type CKey string

type Config struct {
	Validator *validator.Validate
	SecretKey string
}

var (
	instance *Config
)

func App() *Config {
	if instance == nil {
		instance = &Config{
			Validator: validator.New(),
			// the secret key will change every time the application is restarted.
			SecretKey: "asdf1234", //RandomString(8),
		}
	}
	return instance
}
