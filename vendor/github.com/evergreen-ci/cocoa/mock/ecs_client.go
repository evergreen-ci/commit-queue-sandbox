package mock

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsECS "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/cocoa/ecs"
	"github.com/evergreen-ci/utility"
)

// ECSTaskDefinition represents a mock ECS task definition in the global ECS service.
type ECSTaskDefinition struct {
	ARN           string
	Family        *string
	Revision      *int64
	ContainerDefs []ECSContainerDefinition
	MemoryMB      *string
	CPU           *string
	TaskRole      *string
	ExecutionRole *string
	Tags          map[string]string
	Status        *string
	Registered    *time.Time
	Deregistered  *time.Time
}

func newECSTaskDefinition(def *awsECS.RegisterTaskDefinitionInput, rev int) ECSTaskDefinition {
	id := arn.ARN{
		Partition: "aws",
		Service:   "ecs",
		Resource:  fmt.Sprintf("task-definition:%s/%s", utility.FromStringPtr(def.Family), strconv.Itoa(rev)),
	}

	taskDef := ECSTaskDefinition{
		ARN:           id.String(),
		Family:        def.Family,
		Revision:      utility.ToInt64Ptr(int64(rev)),
		CPU:           def.Cpu,
		MemoryMB:      def.Memory,
		TaskRole:      def.TaskRoleArn,
		ExecutionRole: def.ExecutionRoleArn,
		Status:        utility.ToStringPtr(string(types.TaskDefinitionStatusActive)),
		Registered:    utility.ToTimePtr(time.Now()),
	}

	taskDef.Tags = newECSTags(def.Tags)

	for _, containerDef := range def.ContainerDefinitions {
		taskDef.ContainerDefs = append(taskDef.ContainerDefs, newECSContainerDefinition(containerDef))
	}

	return taskDef
}

func (d *ECSTaskDefinition) export() types.TaskDefinition {
	var containerDefs []types.ContainerDefinition
	for _, def := range d.ContainerDefs {
		containerDefs = append(containerDefs, def.export())
	}

	return types.TaskDefinition{
		TaskDefinitionArn:    utility.ToStringPtr(d.ARN),
		Family:               d.Family,
		Revision:             int32(utility.FromInt64Ptr(d.Revision)),
		Cpu:                  d.CPU,
		Memory:               d.MemoryMB,
		TaskRoleArn:          d.TaskRole,
		ExecutionRoleArn:     d.ExecutionRole,
		Status:               types.TaskDefinitionStatus(utility.FromStringPtr(d.Status)),
		ContainerDefinitions: containerDefs,
		RegisteredAt:         d.Registered,
		DeregisteredAt:       d.Deregistered,
	}
}

// ECSContainerDefinition represents a mock ECS container definition in a mock
// ECS task definition.
type ECSContainerDefinition struct {
	Name     *string
	Image    *string
	Command  []string
	MemoryMB *int32
	CPU      int32
	EnvVars  map[string]string
	Secrets  map[string]string
}

func newECSContainerDefinition(def types.ContainerDefinition) ECSContainerDefinition {
	return ECSContainerDefinition{
		Name:     def.Name,
		Image:    def.Image,
		Command:  def.Command,
		MemoryMB: def.Memory,
		CPU:      def.Cpu,
		EnvVars:  newEnvVars(def.Environment),
		Secrets:  newSecrets(def.Secrets),
	}
}

func (d *ECSContainerDefinition) export() types.ContainerDefinition {
	return types.ContainerDefinition{
		Name:        d.Name,
		Image:       d.Image,
		Command:     d.Command,
		Memory:      d.MemoryMB,
		Cpu:         d.CPU,
		Environment: exportEnvVars(d.EnvVars),
		Secrets:     exportSecrets(d.Secrets),
	}
}

// ECSCluster represents a mock ECS cluster running tasks in the global ECS
// service.
type ECSCluster map[string]ECSTask

// ECSTask represents a mock running ECS task within a cluster.
type ECSTask struct {
	ARN               string
	TaskDef           ECSTaskDefinition
	Cluster           *string
	CapacityProvider  *string
	ContainerInstance *string
	Containers        []ECSContainer
	Overrides         *types.TaskOverride
	Group             *string
	ExecEnabled       bool
	Status            string
	GoalStatus        string
	Created           *time.Time
	StopCode          string
	StopReason        *string
	Stopped           *time.Time
	Tags              map[string]string
}

func newECSTask(in *awsECS.RunTaskInput, taskDef ECSTaskDefinition) ECSTask {
	id := arn.ARN{
		Partition: "aws",
		Service:   "ecs",
		Resource:  fmt.Sprintf("task:%s/%s", utility.FromStringPtr(taskDef.Family), strconv.Itoa(int(utility.FromInt64Ptr(taskDef.Revision)))),
	}

	t := ECSTask{
		ARN:              id.String(),
		Cluster:          in.Cluster,
		CapacityProvider: newCapacityProvider(in.CapacityProviderStrategy),
		ExecEnabled:      in.EnableExecuteCommand,
		Group:            in.Group,
		Status:           string(types.DesiredStatusPending),
		GoalStatus:       string(types.DesiredStatusRunning),
		Created:          utility.ToTimePtr(time.Now()),
		TaskDef:          taskDef,
		Overrides:        in.Overrides,
		Tags:             newECSTags(in.Tags),
	}

	for _, containerDef := range taskDef.ContainerDefs {
		t.Containers = append(t.Containers, newECSContainer(containerDef, t))
	}

	return t
}

func (t *ECSTask) export(includeTags bool) types.Task {
	exported := types.Task{
		TaskArn:              utility.ToStringPtr(t.ARN),
		ClusterArn:           t.Cluster,
		CapacityProviderName: t.CapacityProvider,
		EnableExecuteCommand: t.ExecEnabled,
		Group:                t.Group,
		TaskDefinitionArn:    utility.ToStringPtr(t.TaskDef.ARN),
		Overrides:            t.Overrides,
		Cpu:                  t.TaskDef.CPU,
		Memory:               t.TaskDef.MemoryMB,
		LastStatus:           aws.String(t.Status),
		DesiredStatus:        aws.String(t.GoalStatus),
		CreatedAt:            t.Created,
		StopCode:             types.TaskStopCode(t.StopCode),
		StoppedReason:        t.StopReason,
		StoppedAt:            t.Stopped,
	}
	if includeTags {
		exported.Tags = ecs.ExportTags(t.Tags)
	}

	for _, container := range t.Containers {
		exported.Containers = append(exported.Containers, container.export())
	}

	return exported
}

// ECSContainer represents a mock running ECS container within a task.
type ECSContainer struct {
	ARN        string
	TaskARN    *string
	Name       *string
	Image      *string
	CPU        *int32
	MemoryMB   *int32
	Status     string
	GoalStatus string
}

func newECSContainer(def ECSContainerDefinition, task ECSTask) ECSContainer {
	name := utility.FromStringPtr(def.Name)
	if name == "" {
		name = utility.RandomString()
	}
	id := arn.ARN{
		Partition: "aws",
		Service:   "ecs",
		Resource:  fmt.Sprintf("container-definition:%s-%s/%s", utility.FromStringPtr(task.TaskDef.Family), name, strconv.Itoa(int(utility.FromInt64Ptr(task.TaskDef.Revision)))),
	}

	return ECSContainer{
		ARN:        id.String(),
		TaskARN:    utility.ToStringPtr(task.ARN),
		Name:       def.Name,
		Image:      def.Image,
		CPU:        aws.Int32(def.CPU),
		MemoryMB:   def.MemoryMB,
		Status:     string(types.DesiredStatusPending),
		GoalStatus: string(types.DesiredStatusRunning),
	}
}

func (c *ECSContainer) export() types.Container {
	exported := types.Container{
		ContainerArn: utility.ToStringPtr(c.ARN),
		TaskArn:      c.TaskARN,
		Name:         c.Name,
		Image:        c.Image,
		LastStatus:   aws.String(c.Status),
	}

	if c.CPU != nil {
		exported.Cpu = utility.ToStringPtr(strconv.Itoa(int(*c.CPU)))
	}
	if c.MemoryMB != nil {
		exported.Memory = utility.ToStringPtr(strconv.Itoa(int(utility.FromInt32Ptr(c.MemoryMB))))
	}

	return exported
}

func newECSTags(tags []types.Tag) map[string]string {
	converted := map[string]string{}
	for _, t := range tags {
		converted[utility.FromStringPtr(t.Key)] = utility.FromStringPtr(t.Value)
	}
	return converted
}

func newCapacityProvider(providers []types.CapacityProviderStrategyItem) *string {
	if len(providers) == 0 {
		return nil
	}
	// This is just a fake ECS, so it's okay to arbitrarily choose the first
	// capacity provider for convenience.
	return providers[0].CapacityProvider
}

func newEnvVars(envVars []types.KeyValuePair) map[string]string {
	converted := map[string]string{}
	for _, envVar := range envVars {
		converted[utility.FromStringPtr(envVar.Name)] = utility.FromStringPtr(envVar.Value)
	}
	return converted
}

func exportEnvVars(envVars map[string]string) []types.KeyValuePair {
	var exported []types.KeyValuePair
	for k, v := range envVars {
		exported = append(exported, types.KeyValuePair{
			Name:  utility.ToStringPtr(k),
			Value: utility.ToStringPtr(v),
		})
	}
	return exported
}

func newSecrets(secrets []types.Secret) map[string]string {
	converted := map[string]string{}
	for _, secret := range secrets {
		converted[utility.FromStringPtr(secret.Name)] = utility.FromStringPtr(secret.ValueFrom)
	}
	return converted
}

func exportSecrets(secrets map[string]string) []types.Secret {
	var exported []types.Secret
	for k, v := range secrets {
		exported = append(exported, types.Secret{
			Name:      utility.ToStringPtr(k),
			ValueFrom: utility.ToStringPtr(v),
		})
	}
	return exported
}

// ECSService is a global implementation of ECS that provides a simplified
// in-memory implementation of the service that only stores metadata and does
// not orchestrate real containers or container instances. This can be used
// indirectly with the ECSClient to access or modify ECS resources, or used
// directly.
type ECSService struct {
	Clusters map[string]ECSCluster
	TaskDefs map[string][]ECSTaskDefinition
}

// GlobalECSService represents the global fake ECS service state.
var GlobalECSService ECSService

func init() {
	ResetGlobalECSService()
}

// ResetGlobalECSService resets the global fake ECS service back to an
// initialized but clean state.
func ResetGlobalECSService() {
	GlobalECSService = ECSService{
		Clusters: map[string]ECSCluster{},
		TaskDefs: map[string][]ECSTaskDefinition{},
	}
}

// getLatestTaskDefinition is the same as getTaskDefinition, but it can also
// interpret the identifier as just a family name if it's neither an ARN or a
// family and revision. If it matches a family name, the latest active revision
// is returned.
func (s *ECSService) getLatestTaskDefinition(id string) (*ECSTaskDefinition, error) {
	if def, err := s.getTaskDefinition(id); err == nil {
		return def, nil
	}

	// Use the latest active revision in the family if no revision is given.
	family := id
	revisions, ok := GlobalECSService.TaskDefs[family]
	if !ok {
		return nil, errors.New("task definition family not found")
	}

	for i := len(revisions) - 1; i >= 0; i-- {
		if utility.FromStringPtr(revisions[i].Status) == string(types.TaskDefinitionStatusActive) {
			return &revisions[i], nil
		}
	}

	return nil, errors.New("task definition family has no active revisions")
}

// getTaskDefinition gets a task definition by the identifier. The identifier is
// either the task definition's ARN or its family and revision.
func (s *ECSService) getTaskDefinition(id string) (*ECSTaskDefinition, error) {
	if arn.IsARN(id) {
		family, revNum, found := s.taskDefIndexFromARN(id)
		if !found {
			return nil, errors.New("task definition not found")
		}
		return &GlobalECSService.TaskDefs[family][revNum-1], nil
	}

	family, revNum, err := parseFamilyAndRevision(id)
	if err == nil {
		revisions, ok := GlobalECSService.TaskDefs[family]
		if !ok {
			return nil, errors.New("task definition family not found")
		}
		if revNum > len(revisions) {
			return nil, errors.New("task definition revision not found")
		}

		return &revisions[revNum-1], nil
	}

	return nil, errors.New("task definition not found")
}

// parseFamilyAndRevision parses a task definition in the format
// "family:revision".
func parseFamilyAndRevision(taskDef string) (family string, revNum int, err error) {
	partition := strings.LastIndex(taskDef, ":")
	if partition == -1 {
		return "", -1, errors.New("task definition is not in family:revision format")
	}

	family = taskDef[:partition]

	revNum, err = strconv.Atoi(taskDef[partition+1:])
	if err != nil {
		return "", -1, errors.Wrap(err, "parsing revision")
	}
	if revNum <= 0 {
		return "", -1, errors.New("revision cannot be less than 1")
	}

	return family, revNum, nil
}

func (s *ECSService) taskDefIndexFromARN(arn string) (family string, revNum int, found bool) {
	for family, revisions := range GlobalECSService.TaskDefs {
		for revIdx, def := range revisions {
			if def.ARN == arn {
				return family, revIdx + 1, true
			}
		}
	}
	return "", -1, false
}

// ECSClient provides a mock implementation of a cocoa.ECSClient. This makes
// it possible to introspect on inputs to the client and control the client's
// output. It provides some default implementations where possible. By default,
// it will issue the API calls to the fake GlobalECSService.
type ECSClient struct {
	RegisterTaskDefinitionInput  *awsECS.RegisterTaskDefinitionInput
	RegisterTaskDefinitionOutput *awsECS.RegisterTaskDefinitionOutput
	RegisterTaskDefinitionError  error

	DescribeTaskDefinitionInput  *awsECS.DescribeTaskDefinitionInput
	DescribeTaskDefinitionOutput *awsECS.DescribeTaskDefinitionOutput
	DescribeTaskDefinitionError  error

	ListTaskDefinitionsInput  *awsECS.ListTaskDefinitionsInput
	ListTaskDefinitionsOutput *awsECS.ListTaskDefinitionsOutput
	ListTaskDefinitionsError  error

	DeregisterTaskDefinitionInput  *awsECS.DeregisterTaskDefinitionInput
	DeregisterTaskDefinitionOutput *awsECS.DeregisterTaskDefinitionOutput
	DeregisterTaskDefinitionError  error

	RunTaskInput  *awsECS.RunTaskInput
	RunTaskOutput *awsECS.RunTaskOutput
	RunTaskError  error

	DescribeTasksInput  *awsECS.DescribeTasksInput
	DescribeTasksOutput *awsECS.DescribeTasksOutput
	DescribeTasksError  error

	ListTasksInput  *awsECS.ListTasksInput
	ListTasksOutput *awsECS.ListTasksOutput
	ListTasksError  error

	StopTaskInput  *awsECS.StopTaskInput
	StopTaskOutput *awsECS.StopTaskOutput
	StopTaskError  error

	TagResourceInput  *awsECS.TagResourceInput
	TagResourceOutput *awsECS.TagResourceOutput
	TagResourceError  error
}

// RegisterTaskDefinition saves the input and returns a new mock task
// definition. The mock output can be customized. By default, it will create a
// cached task definition based on the input.
func (c *ECSClient) RegisterTaskDefinition(ctx context.Context, in *awsECS.RegisterTaskDefinitionInput) (*awsECS.RegisterTaskDefinitionOutput, error) {
	c.RegisterTaskDefinitionInput = in

	if c.RegisterTaskDefinitionOutput != nil || c.RegisterTaskDefinitionError != nil {
		return c.RegisterTaskDefinitionOutput, c.RegisterTaskDefinitionError
	}

	if in.Family == nil {
		return nil, &types.InvalidParameterException{Message: aws.String("missing family")}
	}

	revisions := GlobalECSService.TaskDefs[utility.FromStringPtr(in.Family)]
	rev := len(revisions) + 1

	taskDef := newECSTaskDefinition(in, rev)

	GlobalECSService.TaskDefs[utility.FromStringPtr(in.Family)] = append(revisions, taskDef)

	exportedTask := taskDef.export()
	return &awsECS.RegisterTaskDefinitionOutput{
		TaskDefinition: &exportedTask,
		Tags:           in.Tags,
	}, nil
}

// DescribeTaskDefinition saves the input and returns information about the
// matching task definition. The mock output can be customized. By default, it
// will return the task definition information if it exists.
func (c *ECSClient) DescribeTaskDefinition(ctx context.Context, in *awsECS.DescribeTaskDefinitionInput) (*awsECS.DescribeTaskDefinitionOutput, error) {
	c.DescribeTaskDefinitionInput = in

	if c.DescribeTaskDefinitionOutput != nil || c.DescribeTaskDefinitionError != nil {
		return c.DescribeTaskDefinitionOutput, c.DescribeTaskDefinitionError
	}

	id := utility.FromStringPtr(in.TaskDefinition)

	def, err := GlobalECSService.getLatestTaskDefinition(id)
	if err != nil {
		return nil, &types.ResourceNotFoundException{Message: aws.String("task definition not found")}
	}

	exportedDef := def.export()
	resp := awsECS.DescribeTaskDefinitionOutput{
		TaskDefinition: &exportedDef,
	}
	include := make([]string, 0, len(in.Include))
	for _, field := range in.Include {
		include = append(include, string(field))
	}
	if shouldIncludeTags(include) {
		resp.Tags = ecs.ExportTags(def.Tags)
	}

	return &resp, nil
}

// ListTaskDefinitions saves the input and lists all matching task definitions.
// The mock output can be customized. By default, it will list all cached task
// definitions that match the input filters.
func (c *ECSClient) ListTaskDefinitions(ctx context.Context, in *awsECS.ListTaskDefinitionsInput) (*awsECS.ListTaskDefinitionsOutput, error) {
	c.ListTaskDefinitionsInput = in

	if c.ListTaskDefinitionsOutput != nil || c.ListTaskDefinitionsError != nil {
		return c.ListTaskDefinitionsOutput, c.ListTaskDefinitionsError
	}

	var arns []string
	for _, revisions := range GlobalECSService.TaskDefs {
		for _, def := range revisions {
			if in.FamilyPrefix != nil && utility.FromStringPtr(def.Family) != *in.FamilyPrefix {
				continue
			}
			if utility.FromStringPtr(def.Status) != string(in.Status) {
				continue
			}

			arns = append(arns, def.ARN)
		}
	}

	return &awsECS.ListTaskDefinitionsOutput{
		TaskDefinitionArns: arns,
	}, nil
}

// DeregisterTaskDefinition saves the input and deletes an existing mock task
// definition. The mock output can be customized. By default, it will delete a
// cached task definition if it exists.
func (c *ECSClient) DeregisterTaskDefinition(ctx context.Context, in *awsECS.DeregisterTaskDefinitionInput) (*awsECS.DeregisterTaskDefinitionOutput, error) {
	c.DeregisterTaskDefinitionInput = in

	if c.DeregisterTaskDefinitionOutput != nil || c.DeregisterTaskDefinitionError != nil {
		return c.DeregisterTaskDefinitionOutput, c.DeregisterTaskDefinitionError
	}

	if in.TaskDefinition == nil {
		return nil, &types.InvalidParameterException{Message: aws.String("missing task definition")}
	}

	id := utility.FromStringPtr(in.TaskDefinition)

	def, err := GlobalECSService.getTaskDefinition(id)
	if err != nil {
		return nil, &types.ResourceNotFoundException{Message: aws.String("task definition not found")}
	}

	def.Status = utility.ToStringPtr(string(types.TaskDefinitionStatusInactive))
	def.Deregistered = utility.ToTimePtr(time.Now())
	GlobalECSService.TaskDefs[utility.FromStringPtr(def.Family)][utility.FromInt64Ptr(def.Revision)-1] = *def

	exportedDef := def.export()
	return &awsECS.DeregisterTaskDefinitionOutput{
		TaskDefinition: &exportedDef,
	}, nil
}

// RunTask saves the input options and returns the mock result of running a task
// definition. The mock output can be customized. By default, it will create
// mock output based on the input.
func (c *ECSClient) RunTask(ctx context.Context, in *awsECS.RunTaskInput) (*awsECS.RunTaskOutput, error) {
	c.RunTaskInput = in

	if c.RunTaskOutput != nil || c.RunTaskError != nil {
		return c.RunTaskOutput, c.RunTaskError
	}

	if in.TaskDefinition == nil {
		return nil, &types.InvalidParameterException{Message: aws.String("missing task definition")}
	}

	clusterName := c.getOrDefaultCluster(in.Cluster)
	cluster, ok := GlobalECSService.Clusters[clusterName]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("cluster not found")}
	}

	taskDefID := utility.FromStringPtr(in.TaskDefinition)

	def, err := GlobalECSService.getLatestTaskDefinition(taskDefID)
	if err != nil {
		return nil, &types.ResourceNotFoundException{Message: aws.String("task definition not found")}
	}

	task := newECSTask(in, *def)

	cluster[task.ARN] = task

	return &awsECS.RunTaskOutput{
		Tasks: []types.Task{task.export(true)},
	}, nil
}

func (c *ECSClient) getOrDefaultCluster(name *string) string {
	if name == nil {
		return "default"
	}
	return *name
}

// DescribeTasks saves the input and returns information about the existing
// tasks. The mock output can be customized. By default, it will describe all
// cached tasks that match.
func (c *ECSClient) DescribeTasks(ctx context.Context, in *awsECS.DescribeTasksInput) (*awsECS.DescribeTasksOutput, error) {
	c.DescribeTasksInput = in

	if c.DescribeTasksOutput != nil || c.DescribeTasksError != nil {
		return c.DescribeTasksOutput, c.DescribeTasksError
	}

	cluster, ok := GlobalECSService.Clusters[c.getOrDefaultCluster(in.Cluster)]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("cluster not found")}
	}

	include := make([]string, 0, len(in.Include))
	for _, field := range in.Include {
		include = append(include, string(field))
	}
	includeTags := shouldIncludeTags(include)

	var tasks []types.Task
	var failures []types.Failure
	for _, id := range in.Tasks {
		task, ok := cluster[id]
		if !ok {
			failures = append(failures, types.Failure{
				Arn: utility.ToStringPtr(id),
				// This reason specifically matches the one returned by ECS when
				// it cannot find the task.
				Reason: utility.ToStringPtr(ecs.ReasonTaskMissing),
			})
			continue
		}

		tasks = append(tasks, task.export(includeTags))
	}

	return &awsECS.DescribeTasksOutput{
		Tasks:    tasks,
		Failures: failures,
	}, nil
}

// shouldIncludeTags returns whether or not the ECS response should include
// resource tags.
func shouldIncludeTags(includes []string) bool {
	for _, include := range includes {
		// "TAGS" is a magic string in the ECS API that indicates that the
		// response should include resource tags.
		if include == "TAGS" {
			return true
		}
	}
	return false
}

// ListTasks saves the input and lists all matching tasks. The mock output can
// be customized. By default, it will list all cached task definitions that
// match the input filters.
func (c *ECSClient) ListTasks(ctx context.Context, in *awsECS.ListTasksInput) (*awsECS.ListTasksOutput, error) {
	c.ListTasksInput = in

	if c.ListTasksOutput != nil || c.ListTasksError != nil {
		return c.ListTasksOutput, c.ListTasksError
	}

	cluster, ok := GlobalECSService.Clusters[c.getOrDefaultCluster(in.Cluster)]
	if !ok {
		return &awsECS.ListTasksOutput{}, nil
	}

	var arns []string
	for arn, task := range cluster {
		if task.GoalStatus != string(in.DesiredStatus) {
			continue
		}

		if in.ContainerInstance != nil && utility.FromStringPtr(task.ContainerInstance) != *in.ContainerInstance {
			continue
		}

		if in.Family != nil && utility.FromStringPtr(task.TaskDef.Family) != *in.Family {
			continue
		}

		arns = append(arns, arn)
	}

	return &awsECS.ListTasksOutput{
		TaskArns: arns,
	}, nil
}

// StopTask saves the input and stops a mock task. The mock output can be
// customized. By default, it will mark a cached task as stopped if it exists
// and is running.
func (c *ECSClient) StopTask(ctx context.Context, in *awsECS.StopTaskInput) (*awsECS.StopTaskOutput, error) {
	c.StopTaskInput = in

	if c.StopTaskOutput != nil || c.StopTaskError != nil {
		return c.StopTaskOutput, c.StopTaskError
	}

	cluster, ok := GlobalECSService.Clusters[c.getOrDefaultCluster(in.Cluster)]
	if !ok {
		return nil, &types.ResourceNotFoundException{Message: aws.String("cluster not found")}
	}

	task, ok := cluster[utility.FromStringPtr(in.Task)]
	if !ok {
		return nil, cocoa.NewECSTaskNotFoundError(utility.FromStringPtr(in.Task))
	}

	task.Status = string(types.DesiredStatusStopped)
	task.GoalStatus = string(types.DesiredStatusStopped)
	task.StopCode = string(types.TaskStopCodeUserInitiated)
	task.StopReason = in.Reason
	task.Stopped = utility.ToTimePtr(time.Now())
	for i := range task.Containers {
		task.Containers[i].Status = string(types.DesiredStatusStopped)
	}

	cluster[utility.FromStringPtr(in.Task)] = task

	exportedTask := task.export(true)
	return &awsECS.StopTaskOutput{
		Task: &exportedTask,
	}, nil
}

// TagResource saves the input and tags a mock task or task definition. The mock
// output can be customized. By default, it will add the tag to the resource if
// it exists.
func (c *ECSClient) TagResource(ctx context.Context, in *awsECS.TagResourceInput) (*awsECS.TagResourceOutput, error) {
	c.TagResourceInput = in

	if c.TagResourceOutput != nil || c.TagResourceError != nil {
		return c.TagResourceOutput, c.TagResourceError
	}

	id := utility.FromStringPtr(in.ResourceArn)

	taskDef, err := GlobalECSService.getTaskDefinition(id)
	if err == nil {
		for k, v := range newECSTags(in.Tags) {
			taskDef.Tags[k] = v
		}
		return &awsECS.TagResourceOutput{}, nil
	}

	for _, cluster := range GlobalECSService.Clusters {
		task, ok := cluster[id]
		if !ok {
			continue
		}
		for k, v := range newECSTags(in.Tags) {
			task.Tags[k] = v
		}
		cluster[id] = task
		return &awsECS.TagResourceOutput{}, nil
	}

	return nil, &types.ResourceNotFoundException{Message: aws.String("task or task definition not found")}
}
