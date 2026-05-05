// Package idempotency provides an HTTP middleware that ensures multiple
// identical requests have the same effect as a single request, by replaying
// a previously stored response keyed off a client-supplied idempotency key.
// This is useful for operations like payment processing where duplicate
// requests would otherwise cause unintended side effects.
package idempotency
