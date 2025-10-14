package ecs

import (
	"github.com/evergreen-ci/cocoa"
)

// TaskStatus represents a status from the ECS API for either a task or a
// container.
type TaskStatus string

// Constants representing ECS task or container states.
const (
	// TaskStatusProvisioning indicates that ECS is performing additional work
	// before launching the task (e.g. provisioning a network interface for
	// AWSVPC).
	TaskStatusProvisioning TaskStatus = "PROVISIONING"
	// TaskStatusPending is a transition state indicating that ECS is waiting
	// for the container agent to act.
	TaskStatusPending TaskStatus = "PENDING"
	// TaskStatusActivating indicates that the task is launched but needs to
	// perform additional work before the task is fully running (e.g. service
	// discovery setup).
	TaskStatusActivating TaskStatus = "ACTIVATING"
	// TaskStatusRunning indicates that the task is running.
	TaskStatusRunning TaskStatus = "RUNNING"
	// TaskStatusDeactivating indicates that the task is preparing to stop but
	// needs to perform additional work first (e.g. deregistering load balancer
	// target groups).
	TaskStatusDeactivating TaskStatus = "DEACTIVATING"
	// TaskStatusStopping is a transition state indicating that ECS is waiting
	// for the container agent to act.
	TaskStatusStopping TaskStatus = "STOPPING"
	// TaskStatusDeprovisioning indicates that the task is no longer running but
	// needs to perform additional work before the task is fully stopped (e.g.
	// detaching the network interface for AWSVPC).
	TaskStatusDeprovisioning TaskStatus = "DEPROVISIONING"
	// TaskStatusStopped indicates that the task is stopped.
	TaskStatusStopped TaskStatus = "STOPPED"
)

// ToCocoaStatus converts a task or container ECS status into its equivalent
// Cocoa status.
func (s TaskStatus) ToCocoaStatus() cocoa.ECSStatus {
	switch s {
	case TaskStatusProvisioning, TaskStatusPending, TaskStatusActivating:
		return cocoa.StatusStarting
	case TaskStatusRunning:
		return cocoa.StatusRunning
	case TaskStatusDeactivating, TaskStatusStopping, TaskStatusDeprovisioning:
		return cocoa.StatusStopping
	case TaskStatusStopped:
		return cocoa.StatusStopped
	default:
		return cocoa.StatusUnknown
	}
}

// Before returns whether or not this task status occurs earlier in the ECS task
// lifecycle than the other task status.
// Docs: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle.html
func (s TaskStatus) Before(other TaskStatus) bool {
	return s.ordinal() < other.ordinal()
}

// After returns whether or not this task status occurs later in the ECS task
// lifecycle than the other task status.
// Docs: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle.html
func (s TaskStatus) After(other TaskStatus) bool {
	return s.ordinal() > other.ordinal()
}

// ordinal returns the ordinal position of the status in the ECS task lifecycle
// flow.
// Docs: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle.html
func (s TaskStatus) ordinal() int {
	switch s {
	case TaskStatusProvisioning:
		return 1
	case TaskStatusPending:
		return 2
	case TaskStatusActivating:
		return 3
	case TaskStatusRunning:
		return 4
	case TaskStatusDeactivating:
		return 5
	case TaskStatusStopping:
		return 6
	case TaskStatusDeprovisioning:
		return 7
	case TaskStatusStopped:
		return 8
	default:
		return -1
	}
}

// ContainerInstanceStatus represents a status from the ECS API for a container
// instance running tasks.
type ContainerInstanceStatus string

// Constants representing ECS container instance states.
const (
	// ContainerInstanceStatusRegistering indicates that the container instance
	// is registering with the cluster. For container instances using AWSVPC
	// trunking, this includes provisioning the trunk elastic network interface.
	ContainerInstanceStatusRegistering ContainerInstanceStatus = "REGISTERING"
	// ContainerInstanceStatusRegistrationFailed indicates that the container
	// instance attempted to register but failed.
	ContainerInstanceStatusRegistrationFailed ContainerInstanceStatus = "REGISTRATION_FAILED"
	// ContainerInstanceStatusActive indicates that the container instance is
	// ready to run tasks. When the container instance is active, ECS can
	// schedule tasks for placement on it.
	ContainerInstanceStatusActive ContainerInstanceStatus = "ACTIVE"
	// ContainerInstanceStatusDeregistering indicates that the container
	// instance is deregistering from the cluster. For container instances using
	// AWSVPC trunking, this includes deprovisioning the trunk elastic network
	// interface.
	ContainerInstanceStatusDeregistering ContainerInstanceStatus = "DEREGISTERING"
	// ContainerInstanceStatusInactive indicates that the container instance has
	// been terminated and deregistered from the cluster.
	ContainerInstanceStatusInactive ContainerInstanceStatus = "INACTIVE"
	// ContainerInstanceStatusDraining indicates that the container instance is
	// running, but ECS will not schedule new tasks for placement on it.
	ContainerInstanceStatusDraining ContainerInstanceStatus = "DRAINING"
)
