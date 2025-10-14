package cocoa

import "context"

// ECSPodDefinitionItem represents an item that can be cached in a
// ECSPodDefinitionCache.
type ECSPodDefinitionItem struct {
	// ID is the unique identifier in ECS for pod definition represented by the
	// item.
	ID string
	// DefinitionOpts are the options used to create the pod definition.
	DefinitionOpts ECSPodDefinitionOptions
}

// ECSPodDefinitionManager manages pod definitions, which are configuration
// templates used to run pods.
type ECSPodDefinitionManager interface {
	// CreatePodDefinition creates a pod definition.
	CreatePodDefinition(ctx context.Context, opts ...ECSPodDefinitionOptions) (*ECSPodDefinitionItem, error)
	// DeletePodDefinition deletes an existing pod definition. Implementations
	// should ensure that deletion is idempotent.
	DeletePodDefinition(ctx context.Context, id string) error
}
