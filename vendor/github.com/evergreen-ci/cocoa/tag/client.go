package tag

import (
	"context"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"

	"github.com/evergreen-ci/cocoa/awsutil"
	"github.com/evergreen-ci/utility"
)

// BasicTagClient provides a cocoa.TagClient implementation that wraps the AWS
// Resource Groups Tagging API. It supports retrying requests using exponential
// backoff and jitter.
type BasicTagClient struct {
	awsutil.BaseClient
	rgt *resourcegroupstaggingapi.Client
}

// NewBasicTagClient creates a new AWS Resource Groups Tagging API
// client from the given options.
func NewBasicTagClient(ctx context.Context, opts awsutil.ClientOptions) (*BasicTagClient, error) {
	c := &BasicTagClient{
		BaseClient: awsutil.NewBaseClient(opts),
	}
	if err := c.setup(ctx); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	return c, nil
}

func (c *BasicTagClient) setup(ctx context.Context) error {
	if c.rgt != nil {
		return nil
	}

	config, err := c.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "initializing config")
	}

	c.rgt = resourcegroupstaggingapi.NewFromConfig(*config)

	return nil
}

// GetResources finds arbitrary AWS resources that match the input filters.
func (c *BasicTagClient) GetResources(ctx context.Context, in *resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
	if err := c.setup(ctx); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *resourcegroupstaggingapi.GetResourcesOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("GetResources", in)
		out, err = c.rgt.GetResources(ctx, in)
		grip.Debug(message.WrapError(err, msg))
		if c.isNonRetryableError(err) {
			return false, err
		}

		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BasicTagClient) isNonRetryableError(err error) bool {
	return utility.MatchesError[*types.InvalidParameterException](err)
}
