package mock

import (
	"context"

	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/utility"
)

// ECSPodDefinitionManager provides a mock implementation of a
// cocoa.ECSPodDefinitionManager backed by another ECS pod definition manager
// implementation.
type ECSPodDefinitionManager struct {
	cocoa.ECSPodDefinitionManager

	CreatePodDefinitionInput  []cocoa.ECSPodDefinitionOptions
	CreatePodDefinitionOutput *cocoa.ECSPodDefinitionItem
	CreatePodDefinitionError  error

	DeletePodDefinitionInput *string
	DeletePodDefinitionError error
}

// NewECSPodDefinitionManager creates a mock ECS pod definition manager backed
// by the given pod definition manager.
func NewECSPodDefinitionManager(m cocoa.ECSPodDefinitionManager) *ECSPodDefinitionManager {
	return &ECSPodDefinitionManager{
		ECSPodDefinitionManager: m,
	}
}

// CreatePodDefinition saves the input and returns a new mock pod definition
// item. The mock output can be customized. By default, it will return the
// result of creating the pod definition in the backing ECS pod definition
// manager.
func (m *ECSPodDefinitionManager) CreatePodDefinition(ctx context.Context, opts ...cocoa.ECSPodDefinitionOptions) (*cocoa.ECSPodDefinitionItem, error) {
	m.CreatePodDefinitionInput = opts

	if m.CreatePodDefinitionOutput != nil {
		return m.CreatePodDefinitionOutput, m.CreatePodDefinitionError
	} else if m.CreatePodDefinitionError != nil {
		return nil, m.CreatePodDefinitionError
	}

	return m.ECSPodDefinitionManager.CreatePodDefinition(ctx, opts...)
}

// DeletePodDefinition saves the input and deletes the mock pod definition. The
// mock output can be customized. By default, it will return the result of
// deleting the pod definition from the backing ECS pod definition manager.
func (m *ECSPodDefinitionManager) DeletePodDefinition(ctx context.Context, id string) error {
	m.DeletePodDefinitionInput = utility.ToStringPtr(id)

	if m.DeletePodDefinitionError != nil {
		return m.DeletePodDefinitionError
	}

	return m.ECSPodDefinitionManager.DeletePodDefinition(ctx, id)
}
