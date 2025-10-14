package cocoa

import "context"

// SecretCache represents an external cache that tracks secrets.
type SecretCache interface {
	// Put adds a new secret with the given name and external resource
	// identifier in the cache.
	Put(ctx context.Context, item SecretCacheItem) error
	// Delete deletes an existing secret with the given external resource
	// identifier from the cache.
	Delete(ctx context.Context, id string) error
	// GetTag returns the name of the tracking tag to use for the secret.
	// Implementations are allowed to return an empty string.
	GetTag() string
}

// SecretCacheItem represents an item that can be cached in a SecretCache.
type SecretCacheItem struct {
	// ID is the unique resource identifier for the stored secret.
	ID string
	// Name is the friendly name of the secret.
	Name string
}
