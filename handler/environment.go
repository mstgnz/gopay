package handler

import (
	"errors"
	"net/http"
)

// errInvalidEnvironment is returned when the caller omits the environment or names one
// that does not exist.
var errInvalidEnvironment = errors.New(`environment query parameter is required and must be "sandbox" or "production"`)

// environmentFromRequest resolves the target environment from the query string.
// Anything other than an exact "sandbox" or "production" is rejected, including an
// absent value: the previous fallback silently sent live cards to the provider's test
// system whenever the parameter was missing or misspelled.
func environmentFromRequest(r *http.Request) (string, error) {
	switch environment := r.URL.Query().Get("environment"); environment {
	case "sandbox", "production":
		return environment, nil
	default:
		return "", errInvalidEnvironment
	}
}
