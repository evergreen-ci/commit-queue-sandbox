package awsutil

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// BaseClient provides various helpers to set up and use AWS clients for various
// services.
type BaseClient struct {
	opts   ClientOptions
	config *aws.Config
}

// NewBaseClient creates a new base AWS client from the client options.
func NewBaseClient(opts ClientOptions) BaseClient {
	return BaseClient{opts: opts}
}

// GetConfig ensures that the config is initialized and returns it.
func (c *BaseClient) GetConfig(ctx context.Context) (*aws.Config, error) {
	if err := c.opts.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid options")
	}

	if c.config != nil {
		return c.config, nil
	}

	config, err := c.opts.GetConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating config")
	}

	c.config = config

	return c.config, nil
}

// GetRetryOptions returns the retry options for the client.
func (c *BaseClient) GetRetryOptions() utility.RetryOptions {
	if c.opts.RetryOpts == nil {
		c.opts.RetryOpts = &utility.RetryOptions{}
	}
	return *c.opts.RetryOpts
}
