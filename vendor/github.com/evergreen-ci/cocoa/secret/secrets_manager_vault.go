package secret

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// BasicSecretsManager provides a cocoa.Vault implementation backed by AWS
// Secrets Manager.
type BasicSecretsManager struct {
	client cocoa.SecretsManagerClient
	cache  cocoa.SecretCache
}

// BasicSecretsManagerOptions are options to create a basic Secrets Manager
// vault that's optionally backed by a cache.
type BasicSecretsManagerOptions struct {
	Client cocoa.SecretsManagerClient
	Cache  cocoa.SecretCache
}

// NewBasicSecretsManagerOptions returns new uninitialized options to create a
// basic Secrets Manager vault.
func NewBasicSecretsManagerOptions() *BasicSecretsManagerOptions {
	return &BasicSecretsManagerOptions{}
}

// SetClient sets the client that the vault uses to communicate with Secrets
// Manager.
func (o *BasicSecretsManagerOptions) SetClient(c cocoa.SecretsManagerClient) *BasicSecretsManagerOptions {
	o.Client = c
	return o
}

// SetCache sets the cache used to track secrets externally.
func (o *BasicSecretsManagerOptions) SetCache(sc cocoa.SecretCache) *BasicSecretsManagerOptions {
	o.Cache = sc
	return o
}

var (
	defaultCacheTrackingTag = "cocoa-tracked"
)

// Validate checks that the required parameters to initialize a Secrets Manager
// vault are given.
func (o *BasicSecretsManagerOptions) Validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(o.Client == nil, "must specify a client")
	if catcher.HasErrors() {
		return catcher.Resolve()
	}

	return nil
}

// NewBasicSecretsManager creates a Vault backed by AWS Secrets Manager.
func NewBasicSecretsManager(opts BasicSecretsManagerOptions) (*BasicSecretsManager, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid options")
	}
	return &BasicSecretsManager{
		client: opts.Client,
		cache:  opts.Cache,
	}, nil
}

// CreateSecret creates a new secret and adds it to the cache if it is using
// one. If the secret already exists, it will return the secret ID without
// modifying the secret value. To update an existing secret, see UpdateValue.
func (m *BasicSecretsManager) CreateSecret(ctx context.Context, s cocoa.NamedSecret) (id string, err error) {
	if err := s.Validate(); err != nil {
		return "", errors.Wrap(err, "invalid secret")
	}
	in := &secretsmanager.CreateSecretInput{
		Name:         s.Name,
		SecretString: s.Value,
	}
	if m.usesCache() {
		// If the secret needs to be cached, we could successfully create a
		// cloud secret but fail to cache it. Adding a tag makes it possible to
		// track whether the secret has been created but has not been
		// successfully cached. In that case, the application can query Secrets
		// Manager for secrets that are tagged as untracked to clean them up.
		in.Tags = ExportTags(map[string]string{m.getCacheTag(): strconv.FormatBool(false)})
	}

	out, err := m.client.CreateSecret(ctx, in)
	if err != nil {
		var resourceExistsError *types.ResourceExistsException
		if errors.As(err, &resourceExistsError) {
			// The secret already exists, so describe it to get the ARN.
			describeOut, err := m.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: s.Name})
			if err != nil {
				return "", errors.Wrap(err, "describing already-existing secret")
			}
			if describeOut == nil || describeOut.ARN == nil {
				return "", errors.New("expected an ID for an already-existing secret in the response, but none was returned from Secrets Manager")
			}
			return *describeOut.ARN, nil
		}
		return "", err
	}
	if out == nil || out.ARN == nil {
		return "", errors.New("expected an ID in the response, but none was returned from Secrets Manager")
	}

	arn := utility.FromStringPtr(out.ARN)

	if !m.usesCache() {
		return arn, nil
	}

	item := cocoa.SecretCacheItem{
		ID:   arn,
		Name: utility.FromStringPtr(s.Name),
	}

	if err := m.cache.Put(ctx, item); err != nil {
		return "", errors.Wrapf(err, "adding secret cache item '%s' named '%s' to cache", item.ID, item.Name)
	}

	// Now that the secret is being tracked in the cache, re-tag it to indicate
	// that it's being tracked.
	if _, err := m.client.TagResource(ctx, &secretsmanager.TagResourceInput{
		SecretId: aws.String(arn),
		Tags:     ExportTags(map[string]string{m.getCacheTag(): strconv.FormatBool(true)}),
	}); err != nil {
		return "", errors.Wrapf(err, "re-tagging secret cache item '%s' named '%s' to indicate that it is tracked", item.ID, item.Name)
	}

	return arn, nil
}

// GetValue returns an existing secret's decrypted value.
func (m *BasicSecretsManager) GetValue(ctx context.Context, id string) (val string, err error) {
	if id == "" {
		return "", errors.New("must specify a non-empty ID")
	}

	out, err := m.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &id})
	if err != nil {
		return "", err
	}
	if out == nil || out.SecretString == nil {
		return "", errors.New("expected a value in the response, but none was returned from Secrets Manager")
	}
	return *out.SecretString, nil
}

// UpdateValue updates an existing secret's value.
func (m *BasicSecretsManager) UpdateValue(ctx context.Context, s cocoa.NamedSecret) error {
	if err := s.Validate(); err != nil {
		return errors.Wrap(err, "invalid secret")
	}
	_, err := m.client.UpdateSecretValue(ctx, &secretsmanager.UpdateSecretInput{
		SecretId:     s.Name,
		SecretString: s.Value,
	})
	return err
}

// DeleteSecret deletes an existing secret and deletes it from the cache if it
// is using one.
func (m *BasicSecretsManager) DeleteSecret(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("must specify a non-empty ID")
	}
	_, err := m.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		ForceDeleteWithoutRecovery: aws.Bool(true),
		SecretId:                   &id,
	})
	if err != nil {
		return err
	}

	if !m.usesCache() {
		return nil
	}

	if err := m.cache.Delete(ctx, id); err != nil {
		return errors.Wrapf(err, "deleting secret '%s' from cache", id)
	}

	return nil
}

func (m *BasicSecretsManager) usesCache() bool {
	return m.cache != nil
}

// getCacheTag returns the configured or default cache tracking tag if it is
// using a cache. if it is not caching, this returns the empty string.
func (m *BasicSecretsManager) getCacheTag() string {
	if !m.usesCache() {
		return ""
	}
	if t := m.cache.GetTag(); t != "" {
		return t
	}
	return defaultCacheTrackingTag
}

// ExportTags converts a mapping of tag names to values into Secrets Manager
// tags.
func ExportTags(tags map[string]string) []types.Tag {
	var smTags []types.Tag

	for k, v := range tags {
		smTags = append(smTags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	return smTags
}
