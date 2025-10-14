package plank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

const defaultBaseURL = "https://logkeeper2.build.10gen.cc"

// LogkeeperClient is a simple read-only API client for the Logkeeper service.
type LogkeeperClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLogkeeperClientOptions specify the arguments for creating a new Logkeeper
// client.
type NewLogkeeperClientOptions struct {
	// BaseURL is the base URL of the Logkeeper service. Defaults to
	// `https://logkeeper2.build.10gen.cc`.
	BaseURL string
	// HTTPClient is the underlying HTTP client for making requests to the
	// Logkeeper service. Optional.
	HTTPClient *http.Client
}

// NewLogkeeperClient returns a new Logkeeper client with the given options.
func NewLogkeeperClient(opts NewLogkeeperClientOptions) *LogkeeperClient {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}

	return &LogkeeperClient{
		baseURL:    opts.BaseURL,
		httpClient: opts.HTTPClient,
	}
}

// GetBuildMetadata returns the metadata for the given Logkeeper build ID.
func (c *LogkeeperClient) GetBuildMetadata(ctx context.Context, buildID string) (Build, error) {
	resp, err := c.get(ctx, fmt.Sprintf("build/%s?metadata=true", buildID))
	if err != nil {
		return Build{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Build{}, errors.Errorf("Logkeeper returned HTTP status %d", resp.StatusCode)
	}

	var build Build
	if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
		return Build{}, errors.Wrap(err, "decoding JSON response")
	}

	return build, nil
}

// GetTestMetadata returns the metadata for the given Logkeeper test ID.
func (c *LogkeeperClient) GetTestMetadata(ctx context.Context, buildID, testID string) (Test, error) {
	resp, err := c.get(ctx, fmt.Sprintf("build/%s/test/%s?metadata=true", buildID, testID))
	if err != nil {
		return Test{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Test{}, errors.Errorf("Logkeeper returned HTTP status %d", resp.StatusCode)
	}

	var test Test
	if err := json.NewDecoder(resp.Body).Decode(&test); err != nil {
		return Test{}, errors.Wrap(err, "decoding JSON response")
	}

	return test, nil
}

// StreamAllLogs returns a stream of all logs from the given Logkeeper build
// ID. It is the responsibility of the caller to close the stream.
func (c *LogkeeperClient) StreamAllLogs(ctx context.Context, buildID string) (io.ReadCloser, error) {
	resp, err := c.get(ctx, fmt.Sprintf("build/%s/all?raw=true", buildID))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Logkeeper returned HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// StreamTestLogs returns a stream of the logs from the given Logkeeper build
// ID and test ID. It is the responsibility of the caller to close the stream.
func (c *LogkeeperClient) StreamTestLogs(ctx context.Context, buildID, testID string) (io.ReadCloser, error) {
	resp, err := c.get(ctx, fmt.Sprintf("build/%s/test/%s?raw=true", buildID, testID))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Logkeeper returned HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (c *LogkeeperClient) get(ctx context.Context, url string) (*http.Response, error) {
	url = fmt.Sprintf("%s/%s", c.baseURL, url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating Logkeeper HTTP request")
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = utility.GetHTTPClient()
		defer utility.PutHTTPClient(httpClient)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "doing Logkeeper HTTP request")
	}

	return resp, nil
}
