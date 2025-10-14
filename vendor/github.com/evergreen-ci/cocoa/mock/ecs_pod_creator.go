package mock

import (
	"context"

	"github.com/evergreen-ci/cocoa"
)

// ECSPodCreator provides a mock implementation of a cocoa.ECSPodCreator
// backed by another ECS pod creator implementation.
type ECSPodCreator struct {
	cocoa.ECSPodCreator

	CreatePodInput  []cocoa.ECSPodCreationOptions
	CreatePodOutput *cocoa.ECSPod
	CreatePodError  error

	CreatePodFromExistingDefinitionInput  []cocoa.ECSPodExecutionOptions
	CreatePodFromExistingDefinitionOutput *cocoa.ECSPod
	CreatePodFromExistingDefinitionError  error
}

// NewECSPodCreator creates a mock ECS pod creator backed by the given pod
// creator.
func NewECSPodCreator(c cocoa.ECSPodCreator) *ECSPodCreator {
	return &ECSPodCreator{
		ECSPodCreator: c,
	}
}

// CreatePod saves the input and returns a new mock pod. The mock output can be
// customized. By default, it will return the result of creating the pod in the
// backing ECS pod creator.
func (m *ECSPodCreator) CreatePod(ctx context.Context, opts ...cocoa.ECSPodCreationOptions) (cocoa.ECSPod, error) {
	m.CreatePodInput = opts

	if m.CreatePodOutput != nil {
		return *m.CreatePodOutput, m.CreatePodError
	} else if m.CreatePodError != nil {
		return nil, m.CreatePodError
	}

	return m.ECSPodCreator.CreatePod(ctx, opts...)
}

// CreatePodFromExistingDefinition saves the input and returns a new mock pod.
// The mock output can be customized. By default, it will return the result of
// creating the pod in the backing ECS pod creator.
func (m *ECSPodCreator) CreatePodFromExistingDefinition(ctx context.Context, def cocoa.ECSTaskDefinition, opts ...cocoa.ECSPodExecutionOptions) (cocoa.ECSPod, error) {
	m.CreatePodFromExistingDefinitionInput = opts

	if m.CreatePodFromExistingDefinitionOutput != nil {
		return *m.CreatePodOutput, m.CreatePodFromExistingDefinitionError
	} else if m.CreatePodError != nil {
		return nil, m.CreatePodError
	}

	return m.ECSPodCreator.CreatePodFromExistingDefinition(ctx, def, opts...)
}
