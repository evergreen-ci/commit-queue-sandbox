package mock

import (
	"context"

	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/utility"
)

// SecretCache provides a mock implementation of a cocoa.SecretCache backed by
// another secret cache implementation.
type SecretCache struct {
	cocoa.SecretCache

	PutInput *cocoa.SecretCacheItem
	PutError error

	DeleteInput *string
	DeleteError error

	Tag *string
}

// NewSecretCache creates a mock secret cache backed by the given secret cache.
func NewSecretCache(sc cocoa.SecretCache) *SecretCache {
	return &SecretCache{
		SecretCache: sc,
	}
}

// Put adds the secret to the mock cache. The mock output can be customized. By
// default, it will return the result of putting the secret in the backing
// secret cache.
func (c *SecretCache) Put(ctx context.Context, item cocoa.SecretCacheItem) error {
	c.PutInput = &item

	if c.PutError != nil {
		return c.PutError
	}

	return c.SecretCache.Put(ctx, item)
}

// Delete removes the secret from the mock cache. The mock output can be
// customized. By default, it will return the result of deleting the secret from
// the backing secret cache.
func (c *SecretCache) Delete(ctx context.Context, id string) error {
	c.DeleteInput = &id

	if c.DeleteError != nil {
		return c.DeleteError
	}

	return c.SecretCache.Delete(ctx, id)
}

// GetTag returns the cache tracking tag. The mock output can be customized. By
// default, it will return the tag from the backing secret cache.
func (c *SecretCache) GetTag() string {
	if c.Tag != nil {
		return utility.FromStringPtr(c.Tag)
	}

	return c.SecretCache.GetTag()
}
