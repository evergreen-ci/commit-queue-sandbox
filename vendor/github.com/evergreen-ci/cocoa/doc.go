/*
Package cocoa provides interfaces to interact with groups of containers (called
pods) backed by container orchestration services. Containers are not managed
individually - they're managed as logical groupings of containers.

The ECSPodCreator provides an abstraction to create pods in AWS ECS without
needing to make direct calls to the API.

The ECSPod is a self-contained unit that allows users to manage their pod
without having to make direct calls to the AWS ECS API.

The ECSPodDefinitionManager provides a means to manage pod definition templates
in AWS ECS without needing to make direct calls to the API. This can be used in
conjunction with a ECSPodDefinitionCache to both manage pod definitions in AWS
ECS and also track these definitions in an external cache.

The ECSClient provides a convenience wrapper around the AWS ECS API. If the
ECSPodCreator and ECSPod do not fulfill your needs, you can instead make calls
directly to the ECS API using this client.

The Vault is an ancillary service for pods that supports interacting with a
dedicated secrets management service. It conveniently integrates with pods to
securely pass secrets into containers. This can be used in conjunction with a
SecretCache to both manage the cloud secrets and also keep track of these
secrets in an external cache.

The SecretsManagerClient provides a convenience wrapper around the AWS Secrets
Manager API. If the Vault does not fulfill your needs, you can instead make
calls directly to the Secrets Manager API using this client.

The TagClient provides a wrapper around the AWS Resource Groups Tagging API.
This can be useful for managing tagged resources across different services, such
as secrets, pod definitions, and pods.
*/
package cocoa
