package awsutil

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

// ClientOptions represent AWS client options such as authentication and making
// requests.
type ClientOptions struct {
	// CredsProvider is a credentials provider, which may be used to either connect to
	// the AWS API directly, or authenticate to STS to retrieve temporary
	// credentials to access the API (if Role is specified).
	// If not specified the AWS SDK will attempt to retrieve one from its credentials chain.
	CredsProvider aws.CredentialsProvider
	// Role is the STS role that should be used to perform authorized actions.
	// If specified, Creds will be used to retrieve temporary credentials from
	// STS.
	Role *string
	// Region is the geographical region where API calls should be made.
	Region *string
	// RetryOpts sets the retry policy for API requests.
	RetryOpts *utility.RetryOptions
	// HTTPClient is the HTTP client to use to make requests.
	// If not specified the AWS SDK's default client will be used.
	HTTPClient config.HTTPClient

	stsClient   *sts.Client
	stsProvider *stscreds.AssumeRoleProvider
}

// NewClientOptions returns new unconfigured client options.
func NewClientOptions() *ClientOptions {
	return &ClientOptions{}
}

// SetCredentialsProvider sets the client's credentials provider.
func (o *ClientOptions) SetCredentialsProvider(creds aws.CredentialsProvider) *ClientOptions {
	o.CredsProvider = creds
	return o
}

// SetRole sets the client's role to assume.
func (o *ClientOptions) SetRole(role string) *ClientOptions {
	o.Role = &role
	return o
}

// SetRegion sets the client's geographical region.
func (o *ClientOptions) SetRegion(region string) *ClientOptions {
	o.Region = &region
	return o
}

// SetRetryOptions sets the client's retry options.
func (o *ClientOptions) SetRetryOptions(opts utility.RetryOptions) *ClientOptions {
	o.RetryOpts = &opts
	return o
}

// SetHTTPClient sets the HTTP client to use.
func (o *ClientOptions) SetHTTPClient(hc config.HTTPClient) *ClientOptions {
	o.HTTPClient = hc
	return o
}

// Validate sets defaults for unspecified options.
func (o *ClientOptions) Validate() error {
	if o.RetryOpts == nil {
		o.RetryOpts = &utility.RetryOptions{}
	}
	o.RetryOpts.Validate()

	return nil
}

var configCache = make(map[string]*aws.Config)

// getAWSConfig fetches an aws.Config for the provided region, httpClient, and credsProvider. The config is cached since the AWS SDK will make a call
// to STS each time config.LoadDefaultConfig is called if a credsProvider is not provided and we're running in Kubernetes.
func getAWSConfig(ctx context.Context, region string, httpClient config.HTTPClient, credsProvider aws.CredentialsProvider) (*aws.Config, error) {
	cachableConfig := httpClient == nil && credsProvider == nil
	if cachableConfig && configCache[region] != nil {
		return configCache[region], nil
	}

	config, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithHTTPClient(httpClient),
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return nil, errors.Wrap(err, "loading default AWS config")
	}
	otelaws.AppendMiddlewares(&config.APIOptions)

	if cachableConfig {
		configCache[region] = &config
	}

	return &config, nil
}

// GetCredentialsProvider retrieves the appropriate credentials provider to use for the client.
func (o *ClientOptions) GetCredentialsProvider(ctx context.Context) (aws.CredentialsProvider, error) {
	if o.Role == nil {
		return o.CredsProvider, nil
	}

	if o.stsProvider != nil {
		return o.stsProvider, nil
	}

	if o.stsClient == nil {
		config, err := getAWSConfig(ctx, utility.FromStringPtr(o.Region), o.HTTPClient, o.CredsProvider)
		if err != nil {
			return nil, errors.Wrap(err, "creating STS config")
		}

		o.stsClient = sts.NewFromConfig(*config)
	}

	o.stsProvider = stscreds.NewAssumeRoleProvider(o.stsClient, *o.Role)

	return o.stsProvider, nil
}

// GetConfig gets the authenticated config to perform authorized API actions.
func (o *ClientOptions) GetConfig(ctx context.Context) (*aws.Config, error) {
	creds, err := o.GetCredentialsProvider(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting credentials")
	}

	config, err := getAWSConfig(ctx, utility.FromStringPtr(o.Region), o.HTTPClient, creds)
	if err != nil {
		return nil, errors.Wrap(err, "creating config")
	}
	return config, nil
}
