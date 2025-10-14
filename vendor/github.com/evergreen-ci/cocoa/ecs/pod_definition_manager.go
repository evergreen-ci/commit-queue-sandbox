package ecs

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// BasicPodDefinitionManager manages pod definitions, which are configuration
// templates used to run pods. It can be optionally backed by an external
// cache to keep track of the pod definitions.
type BasicPodDefinitionManager struct {
	client cocoa.ECSClient
	vault  cocoa.Vault
	cache  cocoa.ECSPodDefinitionCache
}

// BasicPodDefinitionManagerOptions are options to create a basic ECS pod
// definition manager that's optionally backed by a cache.
type BasicPodDefinitionManagerOptions struct {
	Client cocoa.ECSClient
	Vault  cocoa.Vault
	Cache  cocoa.ECSPodDefinitionCache
}

// NewBasicPodDefinitionManagerOptions returns new uninitialized options to
// create a basic pod definition manager.
func NewBasicPodDefinitionManagerOptions() *BasicPodDefinitionManagerOptions {
	return &BasicPodDefinitionManagerOptions{}
}

// SetClient sets the client the pod manager uses to communicate with ECS.
func (o *BasicPodDefinitionManagerOptions) SetClient(c cocoa.ECSClient) *BasicPodDefinitionManagerOptions {
	o.Client = c
	return o
}

// SetVault sets the vault that the pod manager uses to manage secrets.
func (o *BasicPodDefinitionManagerOptions) SetVault(v cocoa.Vault) *BasicPodDefinitionManagerOptions {
	o.Vault = v
	return o
}

// SetCache sets the cache used to track pod definitions externally.
func (o *BasicPodDefinitionManagerOptions) SetCache(pdc cocoa.ECSPodDefinitionCache) *BasicPodDefinitionManagerOptions {
	o.Cache = pdc
	return o
}

var (
	defaultCacheTrackingTag = "cocoa-tracked"
)

// Validate checks that the required parameters to initialize a pod definition
// manager are given.
func (o *BasicPodDefinitionManagerOptions) Validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(o.Client == nil, "must specify a client")
	if catcher.HasErrors() {
		return catcher.Resolve()
	}

	return nil
}

// NewBasicPodDefinitionManager creates a new pod definition manager optionally
// backed by a cache.
func NewBasicPodDefinitionManager(opts BasicPodDefinitionManagerOptions) (*BasicPodDefinitionManager, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid options")
	}
	return &BasicPodDefinitionManager{
		client: opts.Client,
		vault:  opts.Vault,
		cache:  opts.Cache,
	}, nil
}

// CreatePodDefinition creates a pod definition and caches it if it is using a
// cache.
func (m *BasicPodDefinitionManager) CreatePodDefinition(ctx context.Context, opts ...cocoa.ECSPodDefinitionOptions) (*cocoa.ECSPodDefinitionItem, error) {
	mergedOpts := cocoa.MergeECSPodDefinitionOptions(opts...)
	if err := mergedOpts.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid pod definition options")
	}
	if m.usesCache() {
		// If the definition needs to be cached, we could successfully create a
		// cloud pod definition but fail to cache it. Adding a tag makes it
		// possible to track whether the pod definition has been created but
		// has not been successfully cached. In that case, the application can
		// query ECS for pod definitions that are tagged as untracked to clean
		// them up.
		mergedOpts.AddTags(map[string]string{m.getCacheTag(): strconv.FormatBool(false)})
	}

	if err := createSecrets(ctx, m.vault, &mergedOpts); err != nil {
		return nil, errors.Wrap(err, "creating new secrets")
	}

	taskDef, err := registerTaskDefinition(ctx, m.client, mergedOpts)
	if err != nil {
		return nil, errors.Wrap(err, "registering task definition")
	}

	item := cocoa.ECSPodDefinitionItem{
		ID:             utility.FromStringPtr(taskDef.TaskDefinitionArn),
		DefinitionOpts: mergedOpts,
	}

	if !m.usesCache() {
		return &item, nil
	}

	if err := m.cache.Put(ctx, item); err != nil {
		return nil, errors.Wrapf(err, "adding pod definition item '%s' named '%s' to cache", item.ID, utility.FromStringPtr(item.DefinitionOpts.Name))
	}

	// Now that the cloud pod definition is being tracked in the cache, re-tag
	// it to indicate that it's being tracked.
	if _, err := m.client.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: aws.String(item.ID),
		Tags:        ExportTags(map[string]string{m.getCacheTag(): strconv.FormatBool(true)}),
	}); err != nil {
		return nil, errors.Wrapf(err, "re-tagging pod definition item '%s' named '%s' to indicate that it is tracked", item.ID, utility.FromStringPtr(item.DefinitionOpts.Name))
	}

	return &item, nil
}

// DeletePodDefinition deletes a pod definition and deletes it from the cache if
// it is using a cache.
func (m *BasicPodDefinitionManager) DeletePodDefinition(ctx context.Context, id string) error {
	if _, err := m.client.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(id),
	}); err != nil {
		return errors.Wrapf(err, "deregistering task definition '%s'", id)
	}

	if m.usesCache() {
		return errors.Wrapf(m.cache.Delete(ctx, id), "deleting pod definition '%s' from cache", id)
	}

	return nil
}

func (m *BasicPodDefinitionManager) usesCache() bool {
	return m.cache != nil
}

// getCacheTag returns the configured or default cache tracking tag if it is
// using a cache. If it is not caching, this returns the empty string.
func (m *BasicPodDefinitionManager) getCacheTag() string {
	if !m.usesCache() {
		return ""
	}
	if t := m.cache.GetTag(); t != "" {
		return t
	}
	return defaultCacheTrackingTag
}
