package provider

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateConfigFields validates configuration against provided field definitions
func ValidateConfigFields(providerName string, config map[string]string, requiredFields []ConfigField) error {
	for _, field := range requiredFields {
		if !field.Required {
			continue
		}

		value, exists := config[field.Key]
		if !exists {
			return fmt.Errorf("%s: required field '%s' is missing", providerName, field.Key)
		}

		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s: required field '%s' cannot be empty", providerName, field.Key)
		}

		// Type-specific validation
		if err := validateFieldType(providerName, field, value); err != nil {
			return err
		}

		// Pattern validation
		if err := validateFieldPattern(providerName, field, value); err != nil {
			return err
		}

		// Length validation
		if err := validateFieldLength(providerName, field, value); err != nil {
			return err
		}
	}

	return nil
}

// validateFieldType validates field based on its type
func validateFieldType(providerName string, field ConfigField, value string) error {
	switch field.Type {
	case "string":
		// String validation is handled by length checks
		return nil
	case "number":
		// Could add number validation if needed
		return nil
	case "url":
		// Could add URL validation if needed
		return nil
	case "email":
		// Could add email validation if needed
		return nil
	case "boolean":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s: field '%s' must be 'true' or 'false'", providerName, field.Key)
		}
		return nil
	default:
		return nil
	}
}

// validateFieldPattern validates field against regex pattern
func validateFieldPattern(providerName string, field ConfigField, value string) error {
	if field.Pattern == "" {
		return nil
	}

	// Special case for environment field
	if field.Key == "environment" {
		validEnvs := []string{"sandbox", "test", "production"}
		for _, env := range validEnvs {
			if value == env {
				return nil
			}
		}
		return fmt.Errorf("%s: environment must be one of: %s", providerName, strings.Join(validEnvs, ", "))
	}

	// General pattern validation
	matched, err := regexp.MatchString(field.Pattern, value)
	if err != nil {
		return fmt.Errorf("%s: invalid pattern for field '%s': %v", providerName, field.Key, err)
	}

	if !matched {
		return fmt.Errorf("%s: field '%s' does not match required pattern", providerName, field.Key)
	}

	return nil
}

// validateFieldLength validates field length constraints
func validateFieldLength(providerName string, field ConfigField, value string) error {
	if field.MinLength > 0 && len(value) < field.MinLength {
		return fmt.Errorf("%s: field '%s' must be at least %d characters", providerName, field.Key, field.MinLength)
	}

	if field.MaxLength > 0 && len(value) > field.MaxLength {
		return fmt.Errorf("%s: field '%s' must not exceed %d characters", providerName, field.Key, field.MaxLength)
	}

	return nil
}
