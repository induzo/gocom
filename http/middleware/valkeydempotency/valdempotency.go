// Package valkeydempotency provides a Valkey-backed implementation of
// the idempotency.Store interface, plus a thin NewMiddleware wrapper that
// constructs the upstream idempotency middleware around it.
//
// The lock keyspace is namespaced by valkeylock (default prefix "rwlock");
// stored responses are namespaced under "idemresp:" so the two key sets
// cannot collide and are easy to filter with Valkey tooling.
package valkeydempotency
