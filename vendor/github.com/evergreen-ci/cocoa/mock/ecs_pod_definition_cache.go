package mock

import (
	"context"

	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/utility"
)

// ECSPodDefinitionCache provides a mock implementation of a
// cocoa.ECSPodDefinitionCache backed by another ECS pod definition cache
// implementation.
type ECSPodDefinitionCache struct {
	cocoa.ECSPodDefinitionCache

	PutInput *cocoa.ECSPodDefinitionItem
	PutError error

	DeleteInput *string
	DeleteError error

	Tag *string
}

// NewECSPodDefinitionCache creates a mock ECS pod definition cache backed
// by the given pod definition cache.
func NewECSPodDefinitionCache(pdc cocoa.ECSPodDefinitionCache) *ECSPodDefinitionCache {
	return &ECSPodDefinitionCache{
		ECSPodDefinitionCache: pdc,
	}
}

// Put adds the item to the mock cache. The mock output can be customized. By
// default, it will return the result of putting the item in the backing ECS pod
// definition cache.
func (c *ECSPodDefinitionCache) Put(ctx context.Context, item cocoa.ECSPodDefinitionItem) error {
	c.PutInput = &item

	if c.PutError != nil {
		return c.PutError
	}

	return c.ECSPodDefinitionCache.Put(ctx, item)
}

// Delete deletes the pod definition matching the identifier from the mock
// cache. The mock output can be customized. By default, it will return the
// result of deleting the pod definition from the backing ECS pod definition
// cache.
func (c *ECSPodDefinitionCache) Delete(ctx context.Context, id string) error {
	c.DeleteInput = &id

	if c.DeleteError != nil {
		return c.DeleteError
	}

	return c.ECSPodDefinitionCache.Delete(ctx, id)
}

// GetTag returns the cache tracking tag. The mock output can be customized. By
// default, it will return the tag from the backing ECS pod definition cache.
func (c *ECSPodDefinitionCache) GetTag() string {
	if c.Tag != nil {
		return utility.FromStringPtr(c.Tag)
	}

	return c.ECSPodDefinitionCache.GetTag()
}
