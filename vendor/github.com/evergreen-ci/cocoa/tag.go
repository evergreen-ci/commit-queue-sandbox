package cocoa

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
)

// TagClient provides a common interface to interact with a client backed by the
// AWS Resource Groups Tagging API. Implementations must handle retrying and
// backoff.
type TagClient interface {
	// GetResources lists arbitrary AWS resources matching the input.
	GetResources(ctx context.Context, in *resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}
