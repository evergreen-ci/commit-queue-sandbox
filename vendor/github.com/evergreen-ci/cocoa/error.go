package cocoa

import (
	"fmt"

	"github.com/pkg/errors"
)

// ECSTaskNotFoundError indicates that the reason for an error or failure in an
// ECS request is because the task with the specified ARN could not be found.
type ECSTaskNotFoundError struct {
	ARN string
}

// Error returns the formatted error message including the ARN of the task.
func (e *ECSTaskNotFoundError) Error() string {
	return fmt.Sprintf("task '%s' not found", e.ARN)
}

// NewECSTaskNotFoundError returns a new error with the given ARN indicating
// that the task could not be found in ECS.
func NewECSTaskNotFoundError(arn string) *ECSTaskNotFoundError {
	return &ECSTaskNotFoundError{ARN: arn}
}

// IsECSTaskNotFoundError returns whether or not the error is due to not being
// able to find the task in ECS.
func IsECSTaskNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := errors.Cause(err).(*ECSTaskNotFoundError)
	return ok
}
