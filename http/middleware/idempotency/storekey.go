package idempotency

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Validation errors.
var (
	ErrEmptyKey        = errors.New("idempotency key cannot be empty")
	ErrKeyTooLong      = errors.New("idempotency key too long")
	ErrInvalidKeyChars = errors.New("idempotency key contains invalid characters")
)

// validKeyPattern defines allowed characters in idempotency keys.
// Allows alphanumeric, hyphens, underscores, and periods.
var validKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

const maxKeyLength = 255

// validateIdempotencyKey ensures the key is safe and valid.
func validateIdempotencyKey(key string) error {
	if key == "" {
		return fmt.Errorf("validateIdempotencyKey: %w", ErrEmptyKey)
	}

	if len(key) > maxKeyLength {
		return fmt.Errorf(
			"validateIdempotencyKey: %w (max %d characters)",
			ErrKeyTooLong,
			maxKeyLength,
		)
	}

	if !validKeyPattern.MatchString(key) {
		return fmt.Errorf(
			"validateIdempotencyKey: %w (allowed: a-z, A-Z, 0-9, ., _, -)",
			ErrInvalidKeyChars,
		)
	}

	return nil
}

// buildStoreKey creates a composite key that scopes the idempotency key to:
// - User/Tenant (prevents cross-user replay)
// - HTTP Method (prevents cross-method replay)
// - URL Path (prevents cross-endpoint replay)
// - Idempotency Key (user-provided unique identifier)
//
// Format: {userID}:{method}:{path}:{key}
// Example: user123:POST:/api/payment:abc-123
func buildStoreKey(
	req *http.Request,
	idempotencyKey string,
	userIDExtractor UserIDExtractorFn,
) string {
	var parts []string

	// Add user/tenant isolation if available
	if userIDExtractor != nil {
		if userID := userIDExtractor(req); userID != "" {
			parts = append(parts, userID)
		}
	}

	// Add method and normalized path
	method := strings.ToUpper(req.Method)
	path := strings.ToLower(req.URL.Path) // Normalize path casing

	parts = append(parts, method, path, idempotencyKey)

	return strings.Join(parts, ":")
}
