// Package idempotency provides an HTTP middleware for managing idempotency.
// Idempotency ensures that multiple identical requests have the same effect
// as making a single request, which is useful for operations like payment processing
// where duplicate requests could lead to unintended consequences.
// This package is an http middleware that does manage idempotency.
package idempotency
