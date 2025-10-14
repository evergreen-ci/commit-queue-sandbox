package cocoa

import "context"

// ECSPodDefinitionCache represents an external cache that tracks pod
// definitions.
type ECSPodDefinitionCache interface {
	// Put adds a new pod definition item or or updates an existing pod
	// definition item.
	Put(ctx context.Context, item ECSPodDefinitionItem) error
	// Delete deletes by its unique identifier in ECS.
	Delete(ctx context.Context, id string) error
	// GetTag returns the name of the tracking tag to use for the pod
	// definition. Implementations are allowed to return an empty string.
	GetTag() string
}
