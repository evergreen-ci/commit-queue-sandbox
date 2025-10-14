package mock

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/evergreen-ci/utility"
)

// taggedResource represents an arbitrary AWS resource with its tags.
type taggedResource struct {
	ID   string
	Tags map[string]string
}

func exportTagMapping(res taggedResource) types.ResourceTagMapping {
	return types.ResourceTagMapping{
		ResourceARN: utility.ToStringPtr(res.ID),
		Tags:        exportResourceTags(res.Tags),
	}
}

func exportResourceTags(tags map[string]string) []types.Tag {
	var exported []types.Tag
	for k, v := range tags {
		exported = append(exported, types.Tag{
			Key:   utility.ToStringPtr(k),
			Value: utility.ToStringPtr(v),
		})
	}
	return exported
}

// TagClient provides a mock implementation of a cocoa.TagClient. This makes
// it possible to introspect on inputs to the client and control the client's
// output. It provides some default implementations where possible. By default,
// it will issue the API calls to either the fake GlobalECSService for ECS or
// fake GlobalSecretCache for Secrets Manager.
type TagClient struct {
	GetResourcesInput  *resourcegroupstaggingapi.GetResourcesInput
	GetResourcesOutput *resourcegroupstaggingapi.GetResourcesOutput
	GetResourcesError  error
}

// GetResources saves the input and filters for the resources matching the input
// filters. The mock output can be customized. By default, it will search for
// matching secrets in Secrets Manager and task definitions in ECS.
func (c *TagClient) GetResources(ctx context.Context, in *resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
	c.GetResourcesInput = in

	if c.GetResourcesOutput != nil || c.GetResourcesError != nil {
		return c.GetResourcesOutput, c.GetResourcesError
	}

	finders, err := c.getResourceFindersMatchingTypeFilters(in.ResourceTypeFilters)
	if err != nil {
		return nil, err
	}

	allMatches := map[string]taggedResource{}
	for _, f := range finders {
		matches, err := c.getResourcesMatchingTagFilters(f, in.TagFilters)
		if err != nil {
			return nil, err
		}
		for id, match := range matches {
			allMatches[id] = match
		}
	}

	var converted []types.ResourceTagMapping
	for _, match := range allMatches {
		converted = append(converted, exportTagMapping(match))
	}

	return &resourcegroupstaggingapi.GetResourcesOutput{
		ResourceTagMappingList: converted,
	}, nil
}

func (c *TagClient) getResourceFindersMatchingTypeFilters(resourceTypes []string) ([]taggedResourceFinder, error) {
	var matchingAnyResourceType []taggedResourceFinder

	if len(resourceTypes) == 0 {
		// If no resource types are filtered, search all resources.
		for _, resourceFinders := range serviceToResourceFinders {
			matchingAnyResourceType = append(matchingAnyResourceType, resourceFinders...)
		}
		return matchingAnyResourceType, nil
	}

	// In order for a resource to be a match, it must match at least one
	// resource type filter.
	for _, rt := range resourceTypes {
		matchingResourceType := c.getResourceFinders(rt)
		if len(matchingResourceType) == 0 {
			return nil, &types.InvalidParameterException{Message: aws.String(fmt.Sprintf("unsupported resource type '%s'", rt))}
		}

		matchingAnyResourceType = append(matchingAnyResourceType, matchingResourceType...)
	}

	return matchingAnyResourceType, nil
}

func (c *TagClient) getResourcesMatchingTagFilters(f taggedResourceFinder, tagFilters []types.TagFilter) (map[string]taggedResource, error) {
	var matchingAllTags map[string]taggedResource

	if len(tagFilters) != 0 {
		// In order for a resource to be a match, it must match all of the
		// tag filters. In order for a resource to match a tag filter, it
		// must have a tag with an exact matching key and its corresponding
		// value must match one of the possible tag values.
		for _, tf := range tagFilters {
			key := utility.FromStringPtr(tf.Key)
			if key == "" {
				return nil, &types.InvalidParameterException{Message: aws.String("must specify a non-empty key for tag filter")}
			}
			matchingTag := f.getTaggedResources(key, tf.Values)

			if matchingAllTags == nil {
				// Initialize the candidate set of matching resources for
				// this resource type.
				matchingAllTags = matchingTag
			} else {
				// Each matching resource must match all the given tag
				// filters.
				matchingAllTags = c.getSetIntersection(matchingAllTags, matchingTag)
			}
		}
	} else {
		// If there are no tag filters, include all resources of the given
		// resource type.
		matchingAllTags = map[string]taggedResource{}

		for id, res := range f.getAllResources() {
			matchingAllTags[id] = res
		}
	}

	return matchingAllTags, nil
}

func (c *TagClient) getSetIntersection(a, b map[string]taggedResource) map[string]taggedResource {
	intersection := map[string]taggedResource{}
	for k, v := range a {
		if _, ok := b[k]; ok {
			intersection[k] = v
		}
	}
	return intersection
}

// serviceToResourceFinders maps the AWS service name to the taggable resources
// that can be searched.
var serviceToResourceFinders = map[string][]taggedResourceFinder{
	"ecs":            {&ecsTaskDefinitionResourceFinder{}},
	"secretsmanager": {&secretsManagerSecretResourceFinder{}},
}

func (c *TagClient) getResourceFinders(resourceType string) []taggedResourceFinder {
	for service, resourceFinders := range serviceToResourceFinders {
		if service == resourceType {
			return resourceFinders
		}
		for _, f := range resourceFinders {
			if f.name() == resourceType {
				return []taggedResourceFinder{f}
			}
		}
	}
	return nil
}

// taggedResourceFinder can find resources of a particular type by tags. This
// interface can be used to query for resources of a particular type matching
// particular tags.
type taggedResourceFinder interface {
	name() string
	getTaggedResources(key string, value []string) map[string]taggedResource
	getAllResources() map[string]taggedResource
}

type ecsTaskDefinitionResourceFinder struct{}

func (f *ecsTaskDefinitionResourceFinder) name() string {
	return "ecs:task-definition"
}

func (f *ecsTaskDefinitionResourceFinder) getTaggedResources(key string, values []string) map[string]taggedResource {
	res := map[string]taggedResource{}
	for _, family := range GlobalECSService.TaskDefs {
		for _, def := range family {
			if utility.FromStringPtr(def.Status) == string(ecsTypes.TaskDefinitionStatusInactive) {
				continue
			}

			v, ok := def.Tags[key]
			if !ok {
				continue
			}

			if len(values) != 0 && !utility.StringSliceContains(values, v) {
				continue
			}

			res[def.ARN] = f.exportTaskDefinitionTaggedResource(def)
		}
	}
	return res
}

func (f *ecsTaskDefinitionResourceFinder) getAllResources() map[string]taggedResource {
	res := map[string]taggedResource{}
	for _, family := range GlobalECSService.TaskDefs {
		for _, revision := range family {
			res[revision.ARN] = f.exportTaskDefinitionTaggedResource(revision)
		}
	}
	return res
}

func (f *ecsTaskDefinitionResourceFinder) exportTaskDefinitionTaggedResource(def ECSTaskDefinition) taggedResource {
	return taggedResource{
		ID:   def.ARN,
		Tags: def.Tags,
	}
}

type secretsManagerSecretResourceFinder struct{}

func (f *secretsManagerSecretResourceFinder) name() string {
	return "secretsmanager:secret"
}

func (f *secretsManagerSecretResourceFinder) getTaggedResources(key string, values []string) map[string]taggedResource {
	res := map[string]taggedResource{}
	for _, s := range GlobalSecretCache {
		if s.IsDeleted {
			continue
		}

		v, ok := s.Tags[key]
		if !ok {
			continue
		}

		if len(values) != 0 && !utility.StringSliceContains(values, v) {
			continue
		}

		res[s.Name] = f.exportSecretTaggedResource(s)
	}
	return res
}

func (f *secretsManagerSecretResourceFinder) getAllResources() map[string]taggedResource {
	res := map[string]taggedResource{}
	for _, s := range GlobalSecretCache {
		res[s.Name] = f.exportSecretTaggedResource(s)
	}
	return res
}

func (f *secretsManagerSecretResourceFinder) exportSecretTaggedResource(s StoredSecret) taggedResource {
	return taggedResource{
		ID:   s.Name,
		Tags: s.Tags,
	}
}
