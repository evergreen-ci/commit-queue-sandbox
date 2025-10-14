package pail

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	s3Manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const compressionEncoding = "gzip"

// S3Permissions is a type that describes the object canned ACL from S3.
type S3Permissions string

// Valid S3 permissions.
const (
	S3PermissionsPrivate                S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLPrivate))
	S3PermissionsPublicRead             S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLPublicRead))
	S3PermissionsPublicReadWrite        S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLPublicReadWrite))
	S3PermissionsAuthenticatedRead      S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLAuthenticatedRead))
	S3PermissionsAWSExecRead            S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLAwsExecRead))
	S3PermissionsBucketOwnerRead        S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLBucketOwnerRead))
	S3PermissionsBucketOwnerFullControl S3Permissions = S3Permissions(string(s3Types.ObjectCannedACLBucketOwnerFullControl))
)

// Validate checks that the S3Permissions string is valid.
func (p S3Permissions) Validate() error {
	switch p {
	case S3PermissionsPublicRead, S3PermissionsPublicReadWrite:
		return nil
	case S3PermissionsPrivate, S3PermissionsAuthenticatedRead, S3PermissionsAWSExecRead:
		return nil
	case S3PermissionsBucketOwnerRead, S3PermissionsBucketOwnerFullControl:
		return nil
	default:
		return errors.New("invalid S3 permissions type specified")
	}
}

type s3BucketSmall struct {
	s3Bucket
}

type s3BucketLarge struct {
	s3Bucket
	minPartSize int
}

type s3Bucket struct {
	dryRun              bool
	deleteOnPush        bool
	deleteOnPull        bool
	singleFileChecksums bool
	compress            bool
	ifNotExists         bool
	verbose             bool
	batchSize           int
	svc                 *s3.Client
	name                string
	prefix              string
	permissions         S3Permissions
	contentType         string
}

// S3Options support the use and creation of S3 backed buckets.
type S3Options struct {
	// DryRun enables running in a mode that will not execute any
	// operations that modify the bucket.
	DryRun bool
	// DeleteOnSync will delete all objects from the target that do not
	// exist in the destination after the completion of a sync operation
	// (Push/Pull).
	DeleteOnSync bool
	// DeleteOnPush will delete all objects from the target that do not
	// exist in the source after the completion of Push.
	DeleteOnPush bool
	// DeleteOnPull will delete all objects from the target that do not
	// exist in the source after the completion of Pull.
	DeleteOnPull bool
	// Compress enables gzipping of uploaded objects. For downloading, objects
	// that are compressed with gzip are automatically decoded.
	Compress bool
	// UseSingleFileChecksums forces the bucket to checksum files before
	// running uploads and download operation (rather than doing these
	// operations independently.) Useful for large files, particularly in
	// coordination with the parallel sync bucket implementations.
	UseSingleFileChecksums bool
	// Verbose sets the logging mode to "debug".
	Verbose bool
	// MaxRetries sets the number of retry attempts for S3 operations.
	// By default it defers to the AWS SDK's default.
	MaxRetries *int
	// Credentials allows the passing in of explicit AWS credentials. These
	// will override the default credentials chain. (Optional)
	Credentials aws.CredentialsProvider
	// SharedCredentialsFilepath, when not empty, will override the default
	// credentials chain and the Credentials value (see above). (Optional)
	SharedCredentialsFilepath string
	// SharedCredentialsProfile, when not empty, will fetch the given
	// credentials profile from the shared credentials file. (Optional)
	SharedCredentialsProfile string
	// AssumeRoleARN specifies an IAM role ARN. When not empty, it will be
	// used to assume the given role for this session. See
	// `https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html` for
	// more information. (Optional)
	AssumeRoleARN string
	// AssumeRoleOptions provide a mechanism to override defaults by
	// applying changes to the AssumeRoleProvider struct created with this
	// session. This field is ignored if AssumeRoleARN is not set.
	// (Optional)
	AssumeRoleOptions []func(*stscreds.AssumeRoleOptions)
	// Region specifies the AWS region.
	Region string
	// Name specifies the name of the bucket.
	Name string
	// Prefix specifies the prefix to use. (Optional)
	Prefix string
	// Permissions sets the S3 permissions to use for each object. Defaults
	// to FULL_CONTROL. See
	// `https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html`
	// for more information.
	Permissions S3Permissions
	// ContentType sets the standard MIME type of the object data. Defaults
	// to nil. See
	//`https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.17`
	// for more information.
	ContentType string
	// IfNotExists, when set to true, will avoid overwriting an already-existing
	// object at the destination key if it already exists.
	IfNotExists bool
}

// CreateAWSStaticCredentials is a wrapper for creating static AWS credentials.
func CreateAWSStaticCredentials(awsKey, awsPassword, awsToken string) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(awsKey, awsPassword, awsToken)
}

func CreateAWSAssumeRoleCredentials(client *sts.Client, roleARN string, externalID *string) aws.CredentialsProvider {
	return stscreds.NewAssumeRoleProvider(client, roleARN, func(aro *stscreds.AssumeRoleOptions) {
		aro.ExternalID = externalID
	})
}

func (s *s3Bucket) normalizeKey(key string) string { return s.Join(s.prefix, key) }

func (s *s3Bucket) denormalizeKey(key string) string { return consistentTrimPrefix(key, s.prefix) }

func newS3BucketBase(ctx context.Context, client *http.Client, options S3Options) (*s3Bucket, error) {
	if options.Permissions != "" {
		if err := options.Permissions.Validate(); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if (options.DeleteOnPush != options.DeleteOnPull) && options.DeleteOnSync {
		return nil, errors.New("ambiguous delete on sync options set")
	}

	config := configOpts{
		region:                    options.Region,
		maxRetries:                aws.ToInt(options.MaxRetries),
		client:                    client,
		sharedCredentialsFilepath: options.SharedCredentialsFilepath,
		sharedCredentialsProfile:  options.SharedCredentialsProfile,
	}
	cfg, err := getCachedConfig(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "getting AWS config")
	}

	var s3Opts []func(*s3.Options)
	if options.Credentials != nil {
		s3Opts = append(s3Opts, func(opts *s3.Options) {
			opts.Credentials = options.Credentials
		})
	} else if options.AssumeRoleARN != "" {
		s3Opts = append(s3Opts, func(opts *s3.Options) {
			assumeRoleClient := sts.NewFromConfig(*cfg)
			opts.Credentials = stscreds.NewAssumeRoleProvider(assumeRoleClient, options.AssumeRoleARN, options.AssumeRoleOptions...)
		})
	}

	svc := s3.NewFromConfig(*cfg, s3Opts...)

	return &s3Bucket{
		name:                options.Name,
		prefix:              options.Prefix,
		compress:            options.Compress,
		singleFileChecksums: options.UseSingleFileChecksums,
		verbose:             options.Verbose,
		svc:                 svc,
		permissions:         options.Permissions,
		contentType:         options.ContentType,
		dryRun:              options.DryRun,
		batchSize:           1000,
		deleteOnPush:        options.DeleteOnPush || options.DeleteOnSync,
		deleteOnPull:        options.DeleteOnPull || options.DeleteOnSync,
		ifNotExists:         options.IfNotExists,
	}, nil
}

var awsConfigs = struct {
	mutex sync.Mutex
	cache map[configOpts]*aws.Config
}{
	mutex: sync.Mutex{},
	cache: make(map[configOpts]*aws.Config),
}

type configOpts struct {
	region                    string
	maxRetries                int
	sharedCredentialsFilepath string
	sharedCredentialsProfile  string
	expiry                    time.Duration
	client                    *http.Client
}

func getCachedConfig(ctx context.Context, cfgOpts configOpts) (*aws.Config, error) {
	isDefault := cfgOpts.client == nil &&
		cfgOpts.sharedCredentialsFilepath == "" &&
		cfgOpts.sharedCredentialsProfile == ""
	// We completely lock the mutex to ensure that we do not create multiple
	// configurations. This locks it for this read and later in this function
	// when we write the new configuration to the cache. This effectively
	// makes reading + writing atomic.
	awsConfigs.mutex.Lock()
	defer awsConfigs.mutex.Unlock()
	if isDefault && awsConfigs.cache[cfgOpts] != nil {
		return awsConfigs.cache[cfgOpts], nil
	}

	var newCfgOpts []func(*config.LoadOptions) error
	if cfgOpts.maxRetries != 0 {
		newCfgOpts = append(newCfgOpts, config.WithRetryMaxAttempts(cfgOpts.maxRetries))
	}
	if cfgOpts.region != "" {
		newCfgOpts = append(newCfgOpts, config.WithRegion(cfgOpts.region))
	}
	if cfgOpts.client != nil {
		newCfgOpts = append(newCfgOpts, config.WithHTTPClient(cfgOpts.client))
	}
	if cfgOpts.sharedCredentialsFilepath != "" {
		newCfgOpts = append(newCfgOpts, config.WithSharedCredentialsFiles([]string{cfgOpts.sharedCredentialsFilepath}))
	}
	if cfgOpts.sharedCredentialsProfile != "" {
		newCfgOpts = append(newCfgOpts, config.WithSharedConfigProfile(cfgOpts.sharedCredentialsProfile))
	}
	if cfgOpts.expiry != 0 {
		newCfgOpts = append(newCfgOpts, config.WithCredentialsCacheOptions(func(cco *aws.CredentialsCacheOptions) {
			cco.ExpiryWindow = cfgOpts.expiry
		}))
	}

	newCfg, err := config.LoadDefaultConfig(ctx, newCfgOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "creating new session")
	}
	if isDefault {
		awsConfigs.cache[cfgOpts] = &newCfg
	}

	return &newCfg, nil
}

// NewS3Bucket returns a Bucket implementation backed by S3. This
// implementation does not support multipart uploads, if you would like to add
// objects larger than 5 gigabytes see NewS3MultiPartBucket.
func NewS3Bucket(ctx context.Context, options S3Options) (Bucket, error) {
	bucket, err := newS3BucketBase(ctx, nil, options)
	if err != nil {
		return nil, err
	}
	return &s3BucketSmall{s3Bucket: *bucket}, nil
}

// NewS3BucketWithHTTPClient returns a Bucket implementation backed by S3 with
// an existing HTTP client connection. This implementation does not support
// multipart uploads, if you would like to add objects larger than 5
// gigabytes see NewS3MultiPartBucket.
func NewS3BucketWithHTTPClient(ctx context.Context, client *http.Client, options S3Options) (Bucket, error) {
	bucket, err := newS3BucketBase(ctx, client, options)
	if err != nil {
		return nil, err
	}
	return &s3BucketSmall{s3Bucket: *bucket}, nil
}

// NewFastGetS3BucketWithHttpClient does the same thing as NewS3BucketWithHTTPClient,
// but returns the FastGetS3Bucket interface instead.
func NewFastGetS3BucketWithHTTPClient(ctx context.Context, client *http.Client, options S3Options) (FastGetS3Bucket, error) {
	bucket, err := newS3BucketBase(ctx, client, options)
	if err != nil {
		return nil, err
	}
	return &s3BucketSmall{s3Bucket: *bucket}, nil
}

// NewS3MultiPartBucket returns a Bucket implementation backed by S3
// that supports multipart uploads for large objects.
func NewS3MultiPartBucket(ctx context.Context, options S3Options) (Bucket, error) {
	bucket, err := newS3BucketBase(ctx, nil, options)
	if err != nil {
		return nil, err
	}
	// 5MB is the minimum size for a multipart upload, so buffer needs to
	// be at least that big.
	return &s3BucketLarge{s3Bucket: *bucket, minPartSize: 1024 * 1024 * 5}, nil
}

// NewS3MultiPartBucketWithHTTPClient returns a Bucket implementation backed
// by S3 with an existing HTTP client connection that supports multipart
// uploads for large objects.
func NewS3MultiPartBucketWithHTTPClient(ctx context.Context, client *http.Client, options S3Options) (Bucket, error) {
	bucket, err := newS3BucketBase(ctx, client, options)
	if err != nil {
		return nil, err
	}
	// 5MB is the minimum size for a multipart upload, so buffer needs to
	// be at least that big.
	return &s3BucketLarge{s3Bucket: *bucket, minPartSize: 1024 * 1024 * 5}, nil
}

func (s *s3Bucket) String() string { return s.name }

func (s *s3Bucket) Check(ctx context.Context) error {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(s.name),
	}

	_, err := s.svc.HeadBucket(ctx, input)
	// Aside from a 404 Not Found error, HEAD bucket returns a 403
	// Forbidden error. If the latter is the case, that is OK because
	// we know the bucket exists and the given credentials may have
	// access to a sub-bucket. See
	// https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketHEAD.html
	// for more information.
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return errors.Wrap(err, "finding bucket")
			}
		}
	}
	return nil
}

func (s *s3Bucket) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.svc.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.name),
		Key:    aws.String(s.normalizeKey(key)),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, errors.Wrap(err, "getting S3 head object")
	}

	return true, nil
}

func (s *s3Bucket) Join(elems ...string) string { return consistentJoin(elems) }

type smallWriteCloser struct {
	isClosed    bool
	dryRun      bool
	compress    bool
	ifNotExists bool
	verbose     bool
	svc         *s3.Client
	buffer      []byte
	name        string
	ctx         context.Context
	key         string
	permissions S3Permissions
	contentType string
}

type largeWriteCloser struct {
	isCreated      bool
	isClosed       bool
	ifNotExists    bool
	compress       bool
	dryRun         bool
	verbose        bool
	partNumber     int32
	minSize        int
	svc            *s3.Client
	ctx            context.Context
	buffer         []byte
	completedParts []s3Types.CompletedPart
	name           string
	key            string
	permissions    S3Permissions
	contentType    string
	uploadID       string
}

func (w *largeWriteCloser) create() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer create",
		"bucket":    w.name,
		"key":       w.key,
	})

	if !w.dryRun {
		input := &s3.CreateMultipartUploadInput{
			Bucket: aws.String(w.name),
			Key:    aws.String(w.key),
			ACL:    s3Types.ObjectCannedACL(string(w.permissions)),
		}

		// a pointer to an empty string doesn't have the default content-type
		// applied, so we do the check ourselves here.
		if w.contentType != "" {
			input.ContentType = aws.String(w.contentType)
		}

		if w.compress {
			input.ContentEncoding = aws.String(compressionEncoding)
		}

		result, err := w.svc.CreateMultipartUpload(w.ctx, input)
		if err != nil {
			return errors.Wrap(err, "creating a multipart upload")
		}
		w.uploadID = *result.UploadId
	}
	w.isCreated = true
	w.partNumber++
	return nil
}

func (w *largeWriteCloser) complete() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer complete",
		"bucket":    w.name,
		"key":       w.key,
	})

	if !w.dryRun {
		input := &s3.CompleteMultipartUploadInput{
			Bucket: aws.String(w.name),
			Key:    aws.String(w.key),
			MultipartUpload: &s3Types.CompletedMultipartUpload{
				Parts: w.completedParts,
			},
			UploadId: aws.String(w.uploadID),
		}

		if w.ifNotExists {
			input.IfNoneMatch = aws.String("*")
		}

		_, err := w.svc.CompleteMultipartUpload(w.ctx, input)
		if err != nil {
			abortErr := w.abort()
			if abortErr != nil {
				return errors.Wrap(abortErr, "aborting multipart upload")
			}
			return errors.Wrap(err, "completing multipart upload")
		}
	}
	return nil
}

func (w *largeWriteCloser) abort() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer abort",
		"bucket":    w.name,
		"key":       w.key,
	})

	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(w.name),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadID),
	}

	_, err := w.svc.AbortMultipartUpload(w.ctx, input)
	return err
}

func (w *largeWriteCloser) flush() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer flush",
		"bucket":    w.name,
		"key":       w.key,
	})

	if !w.isCreated {
		err := w.create()
		if err != nil {
			return err
		}
	}
	if !w.dryRun {
		input := &s3.UploadPartInput{
			Body:       s3Manager.ReadSeekCloser(strings.NewReader(string(w.buffer))),
			Bucket:     aws.String(w.name),
			Key:        aws.String(w.key),
			PartNumber: aws.Int32(w.partNumber),
			UploadId:   aws.String(w.uploadID),
		}
		result, err := w.svc.UploadPart(w.ctx, input)
		if err != nil {
			abortErr := w.abort()
			if abortErr != nil {
				return errors.Wrap(abortErr, "aborting multipart upload")
			}
			return errors.Wrap(err, "uploading part")
		}
		w.completedParts = append(w.completedParts, s3Types.CompletedPart{
			ETag:       result.ETag,
			PartNumber: aws.Int32(w.partNumber),
		})
	}

	w.buffer = []byte{}
	w.partNumber++
	return nil
}

func (w *smallWriteCloser) Write(p []byte) (int, error) {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "small writer write",
		"bucket":    w.name,
		"key":       w.key,
	})

	if w.isClosed {
		return 0, errors.New("writer already closed")
	}
	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *largeWriteCloser) Write(p []byte) (int, error) {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer write",
		"bucket":    w.name,
		"key":       w.key,
	})

	if w.isClosed {
		return 0, errors.New("writer already closed")
	}
	w.buffer = append(w.buffer, p...)
	if len(w.buffer) > w.minSize {
		err := w.flush()
		if err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *smallWriteCloser) Close() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "small writer close",
		"bucket":    w.name,
		"key":       w.key,
	})

	if w.isClosed {
		return errors.New("writer already closed")
	}
	if w.dryRun {
		return nil
	}

	input := &s3.PutObjectInput{
		Body:   s3Manager.ReadSeekCloser(strings.NewReader(string(w.buffer))),
		Bucket: aws.String(w.name),
		Key:    aws.String(w.key),
		ACL:    s3Types.ObjectCannedACL(string(w.permissions)),
	}

	// a pointer to an empty string doesn't have the default content-type
	// applied, so we do the check ourselves here.
	if w.contentType != "" {
		input.ContentType = aws.String(w.contentType)
	}

	if w.ifNotExists {
		input.IfNoneMatch = aws.String("*")
	}

	if w.compress {
		input.ContentEncoding = aws.String(compressionEncoding)
	}

	_, err := w.svc.PutObject(w.ctx, input)
	return errors.Wrap(err, "copying data to file")

}

func (w *largeWriteCloser) Close() error {
	grip.DebugWhen(w.verbose, message.Fields{
		"type":      "s3",
		"dry_run":   w.dryRun,
		"operation": "large writer close",
		"bucket":    w.name,
		"key":       w.key,
	})

	if w.isClosed {
		return errors.New("writer already closed")
	}
	if len(w.buffer) > 0 || w.partNumber == 0 {
		err := w.flush()
		if err != nil {
			return err
		}
	}
	err := w.complete()
	return err
}

type compressingWriteCloser struct {
	gzipWriter io.WriteCloser
	s3Writer   io.WriteCloser
}

func (w *compressingWriteCloser) Write(p []byte) (int, error) {
	return w.gzipWriter.Write(p)
}

func (w *compressingWriteCloser) Close() error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(w.gzipWriter.Close())
	catcher.Add(w.s3Writer.Close())

	return catcher.Resolve()
}

func (s *s3BucketSmall) Writer(ctx context.Context, key string) (io.WriteCloser, error) {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "small writer",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	writer := &smallWriteCloser{
		name:        s.name,
		svc:         s.svc,
		ctx:         ctx,
		key:         s.normalizeKey(key),
		permissions: s.permissions,
		contentType: s.contentType,
		dryRun:      s.dryRun,
		compress:    s.compress,
		ifNotExists: s.ifNotExists,
	}
	if s.compress {
		return &compressingWriteCloser{
			gzipWriter: gzip.NewWriter(writer),
			s3Writer:   writer,
		}, nil
	}
	return writer, nil
}

func (s *s3BucketLarge) Writer(ctx context.Context, key string) (io.WriteCloser, error) {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "large writer",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	writer := &largeWriteCloser{
		minSize:     s.minPartSize,
		name:        s.name,
		svc:         s.svc,
		ctx:         ctx,
		key:         s.normalizeKey(key),
		permissions: s.permissions,
		contentType: s.contentType,
		dryRun:      s.dryRun,
		compress:    s.compress,
		ifNotExists: s.ifNotExists,
		verbose:     s.verbose,
	}
	if s.compress {
		return &compressingWriteCloser{
			gzipWriter: gzip.NewWriter(writer),
			s3Writer:   writer,
		}, nil
	}
	return writer, nil
}

func (s *s3Bucket) Reader(ctx context.Context, key string) (io.ReadCloser, error) {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "reader",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.name),
		Key:    aws.String(s.normalizeKey(key)),
	}

	result, err := s.svc.GetObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" {
				return nil, MakeKeyNotFoundError(err)
			}
		}
		return nil, err
	}
	if aws.ToString(result.ContentEncoding) == "gzip" {
		return gzip.NewReader(result.Body)
	}

	return result.Body, nil
}

func putHelper(ctx context.Context, b *s3Bucket, key string, r io.Reader) error {
	if b.dryRun {
		return nil
	}

	if b.compress {
		var buf bytes.Buffer

		w := gzip.NewWriter(&buf)

		if _, err := io.Copy(w, r); err != nil {
			return errors.Wrap(err, "gzipping file")
		}

		if err := w.Close(); err != nil {
			return errors.Wrap(err, "closing gzip writer")
		}

		r = bytes.NewReader(buf.Bytes())
	}

	uploader := s3Manager.NewUploader(b.svc)
	uploader.Concurrency = getManagerConcurrency()

	key = b.normalizeKey(key)

	input := &s3.PutObjectInput{
		ChecksumAlgorithm: s3Types.ChecksumAlgorithmCrc32,
		Body:              s3Manager.ReadSeekCloser(r),
		Bucket:            aws.String(b.name),
		Key:               aws.String(key),
		ACL:               s3Types.ObjectCannedACL(string(b.permissions)),
	}

	if b.contentType != "" {
		input.ContentType = aws.String(b.contentType)
	}

	if b.ifNotExists {
		input.IfNoneMatch = aws.String("*")
	}

	if b.compress {
		input.ContentEncoding = aws.String(compressionEncoding)
	}

	if _, err := uploader.Upload(ctx, input); err != nil {
		return errors.Wrapf(err, "uploading file")
	}

	return nil

}

func (s *s3BucketSmall) Put(ctx context.Context, key string, r io.Reader) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "put",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	return putHelper(ctx, &s.s3Bucket, key, r)
}

func (s *s3BucketLarge) Put(ctx context.Context, key string, r io.Reader) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "put",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	return putHelper(ctx, &s.s3Bucket, key, r)
}

func (s *s3Bucket) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "get",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	return s.Reader(ctx, key)
}

func getManagerConcurrency() int {
	// After quite a bit of testing, a minimum of 10 seems to perform the best,
	// even on distros with fewer than 10 cores. 10 is also what the AWS cli
	// defaults to. See DEVPROD-16611 for more information on this testing.
	const minConcurrency = 10

	return max(runtime.NumCPU(), minConcurrency)
}

// GetToWriter fetches the key from this bucket and writes the contents to
// an io.WriterAt in parallel. This function uses the s3Manager.Downloader
// API to download and write to this writer in parallel using byte ranges. This
// method is significantly more efficient at fetching large files than Get.
func (s *s3Bucket) GetToWriter(ctx context.Context, key string, w io.WriterAt) error {
	downloader := s3Manager.NewDownloader(s.svc)
	downloader.Concurrency = getManagerConcurrency()

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.name),
		Key:    aws.String(s.normalizeKey(key)),
	}

	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "GetToWriter",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	if _, err := downloader.Download(ctx, w, input); err != nil {
		return errors.Wrapf(err, "downloading file")
	}

	return nil
}

func (s *s3Bucket) s3WithUploadChecksumHelper(ctx context.Context, target, file string) (bool, error) {
	localmd5, err := utility.MD5SumFile(file)
	if err != nil {
		return false, errors.Wrapf(err, "checksumming '%s'", file)
	}
	input := &s3.HeadObjectInput{
		Bucket:  aws.String(s.name),
		Key:     aws.String(target),
		IfMatch: aws.String(localmd5),
	}
	_, err = s.svc.HeadObject(ctx, input)
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "PreconditionFailed" || apiErr.ErrorCode() == "NotFound" {
			return true, nil
		}
	}

	return false, errors.Wrapf(err, "checking if object '%s' exists", target)
}

func doUpload(ctx context.Context, b Bucket, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "opening file '%s'", path)
	}
	defer f.Close()

	return errors.WithStack(b.Put(ctx, key, f))
}

func (s *s3Bucket) uploadHelper(ctx context.Context, b Bucket, key, path string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "upload",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
		"path":          path,
	})

	if s.singleFileChecksums {
		shouldUpload, err := s.s3WithUploadChecksumHelper(ctx, key, path)
		if err != nil {
			return errors.WithStack(err)
		}
		if !shouldUpload {
			return nil
		}
	}

	return errors.WithStack(doUpload(ctx, b, key, path))
}

func (s *s3BucketLarge) Upload(ctx context.Context, key, path string) error {
	return s.uploadHelper(ctx, s, key, path)
}

func (s *s3BucketSmall) Upload(ctx context.Context, key, path string) error {
	return s.uploadHelper(ctx, s, key, path)
}

func doDownload(ctx context.Context, b Bucket, key, path string) error {
	reader, err := b.Reader(ctx, key)
	if err != nil {
		return errors.WithStack(err)
	}

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.Wrapf(err, "creating enclosing directory for file '%s'", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "creating file '%s'", path)
	}
	_, err = io.Copy(f, reader)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "copying data")
	}

	return errors.WithStack(f.Close())
}

func s3DownloadWithChecksum(ctx context.Context, b Bucket, item BucketItem, local string) error {
	localmd5, err := utility.MD5SumFile(local)
	if os.IsNotExist(errors.Cause(err)) {
		if err = doDownload(ctx, b, item.Name(), local); err != nil {
			return errors.WithStack(err)
		}
	} else if err != nil {
		return errors.WithStack(err)
	}
	if localmd5 != item.Hash() {
		if err = doDownload(ctx, b, item.Name(), local); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (s *s3Bucket) downloadHelper(ctx context.Context, b Bucket, key, path string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "download",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
		"path":          path,
	})

	if s.singleFileChecksums {
		iter, err := s.listHelper(ctx, b, s.normalizeKey(key))
		if err != nil {
			return errors.WithStack(err)
		}
		if !iter.Next(ctx) {
			return errors.New("no results found")
		}
		return s3DownloadWithChecksum(ctx, b, iter.Item(), path)
	}

	return doDownload(ctx, b, key, path)
}

func (s *s3BucketSmall) Download(ctx context.Context, key, path string) error {
	return s.downloadHelper(ctx, s, key, path)
}

func (s *s3BucketLarge) Download(ctx context.Context, key, path string) error {
	return s.downloadHelper(ctx, s, key, path)
}

func (s *s3Bucket) pushHelper(ctx context.Context, b Bucket, opts SyncOptions) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "push",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"remote":        opts.Remote,
		"local":         opts.Local,
		"exclude":       opts.Exclude,
	})

	var re *regexp.Regexp
	var err error
	if opts.Exclude != "" {
		re, err = regexp.Compile(opts.Exclude)
		if err != nil {
			return errors.Wrap(err, "compiling exclude regex")
		}
	}

	files, err := walkLocalTree(ctx, opts.Local)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, fn := range files {
		if re != nil && re.MatchString(fn) {
			continue
		}

		target := s.Join(opts.Remote, fn)
		file := filepath.Join(opts.Local, fn)
		shouldUpload, err := s.s3WithUploadChecksumHelper(ctx, target, file)
		if err != nil {
			return errors.WithStack(err)
		}
		if !shouldUpload {
			continue
		}
		if err = doUpload(ctx, b, target, file); err != nil {
			return errors.WithStack(err)
		}
	}

	if s.deleteOnPush && !s.dryRun {
		return errors.Wrap(deleteOnPush(ctx, files, opts.Remote, b), "deleting on sync after push")
	}
	return nil
}

func (s *s3BucketSmall) Push(ctx context.Context, opts SyncOptions) error {
	return s.pushHelper(ctx, s, opts)
}
func (s *s3BucketLarge) Push(ctx context.Context, opts SyncOptions) error {
	return s.pushHelper(ctx, s, opts)
}

func (s *s3Bucket) pullHelper(ctx context.Context, b Bucket, opts SyncOptions) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "pull",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"remote":        opts.Remote,
		"local":         opts.Local,
		"exclude":       opts.Exclude,
	})

	var re *regexp.Regexp
	var err error
	if opts.Exclude != "" {
		re, err = regexp.Compile(opts.Exclude)
		if err != nil {
			return errors.Wrap(err, "compiling exclude regex")
		}
	}

	iter, err := b.List(ctx, opts.Remote)
	if err != nil {
		return errors.WithStack(err)
	}

	keys := []string{}
	for iter.Next(ctx) {
		if iter.Err() != nil {
			return errors.Wrap(err, "iterating bucket")
		}

		if re != nil && re.MatchString(iter.Item().Name()) {
			continue
		}

		localName, err := filepath.Rel(opts.Remote, iter.Item().Name())
		if err != nil {
			return errors.Wrap(err, "getting relative filepath")
		}
		keys = append(keys, localName)

		if err := s3DownloadWithChecksum(ctx, b, iter.Item(), filepath.Join(opts.Local, localName)); err != nil {
			return errors.WithStack(err)
		}
	}

	if s.deleteOnPull && !s.dryRun {
		return errors.Wrap(deleteOnPull(ctx, keys, opts.Local), "deleting on sync after pull")
	}
	return nil
}

func (s *s3BucketSmall) Pull(ctx context.Context, opts SyncOptions) error {
	return s.pullHelper(ctx, s, opts)
}

func (s *s3BucketLarge) Pull(ctx context.Context, opts SyncOptions) error {
	return s.pullHelper(ctx, s, opts)
}

func (s *s3Bucket) Copy(ctx context.Context, options CopyOptions) error {
	if !options.IsDestination {
		options.IsDestination = true
		options.SourceKey = s.Join(s.name, s.normalizeKey(options.SourceKey))
		return options.DestinationBucket.Copy(ctx, options)
	}

	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "copy",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"source_key":    options.SourceKey,
		"dest_key":      options.DestinationKey,
	})

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.name),
		CopySource: aws.String(options.SourceKey),
		Key:        aws.String(s.normalizeKey(options.DestinationKey)),
		ACL:        s3Types.ObjectCannedACL(string(s.permissions)),
	}

	if !s.dryRun {
		_, err := s.svc.CopyObject(ctx, input)
		if err != nil {
			return errors.Wrap(err, "copying data")
		}
	}
	return nil
}

func (s *s3Bucket) Remove(ctx context.Context, key string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"key":           key,
	})

	if !s.dryRun {
		input := &s3.DeleteObjectInput{
			Bucket: aws.String(s.name),
			Key:    aws.String(s.normalizeKey(key)),
		}

		_, err := s.svc.DeleteObject(ctx, input)
		if err != nil {
			return errors.Wrap(err, "removing data")
		}
	}
	return nil
}

func (s *s3Bucket) deleteObjectsWrapper(ctx context.Context, toDelete *s3Types.Delete) error {
	if len(toDelete.Objects) > 0 {
		input := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.name),
			Delete: toDelete,
		}
		_, err := s.svc.DeleteObjects(ctx, input)
		if err != nil {
			return errors.Wrap(err, "removing data")
		}
	}
	return nil
}

func (s *s3Bucket) RemoveMany(ctx context.Context, keys ...string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"keys":          keys,
	})

	catcher := grip.NewBasicCatcher()
	if !s.dryRun {
		count := 0
		toDelete := &s3Types.Delete{}
		for _, key := range keys {
			// Key limit for s3.DeleteObjects, call function and reset.
			if count == s.batchSize {
				catcher.Add(s.deleteObjectsWrapper(ctx, toDelete))
				count = 0
				toDelete = &s3Types.Delete{}
			}
			toDelete.Objects = append(
				toDelete.Objects,
				s3Types.ObjectIdentifier{Key: aws.String(s.normalizeKey(key))},
			)
			count++
		}
		catcher.Add(s.deleteObjectsWrapper(ctx, toDelete))
	}
	return catcher.Resolve()
}

func (s *s3BucketSmall) RemovePrefix(ctx context.Context, prefix string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove prefix",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"prefix":        prefix,
	})

	return removePrefix(ctx, prefix, s)
}

func (s *s3BucketLarge) RemovePrefix(ctx context.Context, prefix string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove prefix",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"prefix":        prefix,
	})

	return removePrefix(ctx, prefix, s)
}

func (s *s3BucketSmall) RemoveMatching(ctx context.Context, expression string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove matching",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"expression":    expression,
	})

	return removeMatching(ctx, expression, s)
}

// MoveObjects moves multiple objects from sourceKeys in this bucket to destKeys in another bucket specified by destBucket.
// The lengths of sourceKeys and destKeys must match.
func (s *s3Bucket) MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error {
	if len(sourceKeys) != len(destKeys) {
		return errors.New("sourceKeys and destKeys must have the same length")
	}
	catcher := grip.NewBasicCatcher()
	var objectsToDelete []s3Types.ObjectIdentifier
	for i, srcKey := range sourceKeys {
		dstKey := destKeys[i]
		grip.DebugWhen(s.verbose, message.Fields{
			"type":          "s3",
			"dry_run":       s.dryRun,
			"operation":     "move",
			"source_bucket": s.name,
			"dest_bucket":   destBucket.String(),
			"source_key":    srcKey,
			"dest_key":      dstKey,
		})
		if s.dryRun {
			continue
		}
		copyOpts := CopyOptions{
			SourceKey:         srcKey,
			DestinationKey:    dstKey,
			DestinationBucket: destBucket,
			IsDestination:     false,
		}
		err := s.Copy(ctx, copyOpts)
		if err != nil {
			catcher.Add(errors.Wrapf(err, "copying object to destination bucket as %s", dstKey))
			continue
		}
		objectsToDelete = append(objectsToDelete, s3Types.ObjectIdentifier{Key: aws.String(s.normalizeKey(srcKey))})
	}
	// Batch delete all successfully copied source objects
	if !s.dryRun && len(objectsToDelete) > 0 {
		input := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.name),
			Delete: &s3Types.Delete{Objects: objectsToDelete},
		}
		_, err := s.svc.DeleteObjects(ctx, input)
		if err != nil {
			catcher.Add(errors.Wrap(err, "batch deleting original objects after transfer"))
		}
	}
	return catcher.Resolve()
}

func (s *s3BucketSmall) MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error {
	return s.s3Bucket.MoveObjects(ctx, destBucket, sourceKeys, destKeys)
}

func (s *s3BucketLarge) MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error {
	return s.s3Bucket.MoveObjects(ctx, destBucket, sourceKeys, destKeys)
}

func (s *s3BucketLarge) RemoveMatching(ctx context.Context, expression string) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "remove matching",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"expression":    expression,
	})

	return removeMatching(ctx, expression, s)
}

func (s *s3Bucket) listHelper(ctx context.Context, b Bucket, prefix string) (BucketIterator, error) {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "list",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"prefix":        prefix,
	})

	contents, isTruncated, err := getObjectsWrapper(ctx, s, prefix, "")
	if err != nil {
		return nil, err
	}
	return &s3BucketIterator{
		contents:    contents,
		idx:         -1,
		isTruncated: isTruncated,
		s:           s,
		b:           b,
		prefix:      prefix,
	}, nil
}

func (s *s3BucketSmall) List(ctx context.Context, prefix string) (BucketIterator, error) {
	return s.listHelper(ctx, s, s.normalizeKey(prefix))
}

func (s *s3BucketLarge) List(ctx context.Context, prefix string) (BucketIterator, error) {
	return s.listHelper(ctx, s, s.normalizeKey(prefix))
}

func getObjectsWrapper(ctx context.Context, s *s3Bucket, prefix, marker string) ([]s3Types.Object, bool, error) {
	input := &s3.ListObjectsInput{
		Bucket: aws.String(s.name),
		Prefix: aws.String(prefix),
		Marker: aws.String(marker),
	}

	result, err := s.svc.ListObjects(ctx, input)
	if err != nil {
		return nil, false, errors.Wrap(err, "listing objects")
	}
	return result.Contents, *result.IsTruncated, nil
}

type s3BucketIterator struct {
	contents    []s3Types.Object
	idx         int
	isTruncated bool
	err         error
	item        *bucketItemImpl
	s           *s3Bucket
	b           Bucket
	prefix      string
}

func (iter *s3BucketIterator) Err() error { return iter.err }

func (iter *s3BucketIterator) Item() BucketItem { return iter.item }

func (iter *s3BucketIterator) Next(ctx context.Context) bool {
	iter.idx++
	if iter.idx > len(iter.contents)-1 {
		if iter.isTruncated {
			contents, isTruncated, err := getObjectsWrapper(
				ctx,
				iter.s,
				iter.prefix,
				*iter.contents[iter.idx-1].Key,
			)
			if err != nil {
				iter.err = err
				return false
			}
			iter.contents = contents
			iter.idx = 0
			iter.isTruncated = isTruncated
		} else {
			return false
		}
	}

	iter.item = &bucketItemImpl{
		bucket: iter.s.name,
		key:    iter.s.denormalizeKey(*iter.contents[iter.idx].Key),
		hash:   strings.Trim(*iter.contents[iter.idx].ETag, `"`),
		b:      iter.b,
	}
	return true
}

type s3ArchiveBucket struct {
	*s3BucketLarge
}

// NewS3ArchiveBucket returns a SyncBucket implementation backed by S3 that
// supports syncing the local file system as a single archive file in S3 rather
// than creating an individual object for each file. This SyncBucket is not
// compatible with regular Bucket implementations.
func NewS3ArchiveBucket(ctx context.Context, options S3Options) (SyncBucket, error) {
	bucket, err := NewS3MultiPartBucket(ctx, options)
	if err != nil {
		return nil, err
	}
	return newS3ArchiveBucketWithMultiPart(bucket)
}

// NewS3ArchiveBucketWithHTTPClient is the same as NewS3ArchiveBucket but
// allows the user to specify an existing HTTP client connection.
func NewS3ArchiveBucketWithHTTPClient(ctx context.Context, client *http.Client, options S3Options) (SyncBucket, error) {
	bucket, err := NewS3MultiPartBucketWithHTTPClient(ctx, client, options)
	if err != nil {
		return nil, err
	}
	return newS3ArchiveBucketWithMultiPart(bucket)
}

func newS3ArchiveBucketWithMultiPart(bucket Bucket) (*s3ArchiveBucket, error) {
	largeBucket, ok := bucket.(*s3BucketLarge)
	if !ok {
		return nil, errors.New("bucket is not a large multipart bucket")
	}
	return &s3ArchiveBucket{s3BucketLarge: largeBucket}, nil
}

const syncArchiveName = "archive.tar"

// Push pushes the contents from opts.Local to the archive prefixed by
// opts.Remote. This operation automatically performs DeleteOnSync in the
// remote regardless of the bucket setting. UseSingleFileChecksums is ignored
// if it is set on the bucket.
func (s *s3ArchiveBucket) Push(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"dry_run":       s.dryRun,
		"operation":     "push",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"remote":        opts.Remote,
		"local":         opts.Local,
		"exclude":       opts.Exclude,
	})

	var re *regexp.Regexp
	var err error
	if opts.Exclude != "" {
		re, err = regexp.Compile(opts.Exclude)
		if err != nil {
			return errors.Wrap(err, "compiling exclude regex")
		}
	}

	files, err := walkLocalTree(ctx, opts.Local)
	if err != nil {
		return errors.WithStack(err)
	}

	target := s.Join(opts.Remote, syncArchiveName)

	s3Writer, err := s.Writer(ctx, target)
	if err != nil {
		return errors.Wrap(err, "creating writer")
	}
	defer s3Writer.Close()

	tarWriter := tar.NewWriter(s3Writer)
	defer tarWriter.Close()

	for _, fn := range files {
		if re != nil && re.MatchString(fn) {
			continue
		}

		file := filepath.Join(opts.Local, fn)
		// We can't compare the checksum without processing all the
		// local matched files as a tar stream, so just upload it
		// unconditionally.
		if err := tarFile(tarWriter, opts.Local, fn); err != nil {
			return errors.Wrap(err, file)
		}
	}

	return nil
}

// Push pulls the contents from the archive prefixed by opts.Remote to
// opts.Local. UseSingleFileChecksums is ignored if it is set on the bucket.
func (s *s3ArchiveBucket) Pull(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(s.verbose, message.Fields{
		"type":          "s3",
		"operation":     "pull",
		"bucket":        s.name,
		"bucket_prefix": s.prefix,
		"remote":        opts.Remote,
		"local":         opts.Local,
		"exclude":       opts.Exclude,
	})

	var re *regexp.Regexp
	var err error
	if opts.Exclude != "" {
		re, err = regexp.Compile(opts.Exclude)
		if err != nil {
			return errors.Wrap(err, "compiling exclude regex")
		}
	}

	target := s.Join(opts.Remote, syncArchiveName)
	reader, err := s.Get(ctx, target)
	if err != nil {
		return errors.Wrapf(err, "getting archive from remote path '%s'", opts.Remote)
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	if err := untar(tarReader, opts.Local, re); err != nil {
		return errors.Wrapf(err, "unarchiving from remote path '%s' to local path '%s'", opts.Remote, opts.Local)
	}

	return nil
}

// PresignExpireTime sets the amount of time the link is live before expiring.
const PresignExpireTime = 24 * time.Hour

// PreSignRequestParams holds all the parameters needed to sign a URL or fetch S3 object metadata.
type PreSignRequestParams struct {
	Bucket                string
	FileKey               string
	Region                string
	SignatureExpiryWindow time.Duration

	// Static credentials specific fields.
	AWSKey          string
	AWSSecret       string
	AWSSessionToken string

	// AssumeRole specific fields.
	AWSRoleARN string
	ExternalID *string
}

func (p *PreSignRequestParams) getS3Client(ctx context.Context) (*s3.Client, error) {
	region := p.Region
	if region == "" {
		region = "us-east-1"
	}

	cfgOpts := configOpts{
		region: region,
		expiry: p.SignatureExpiryWindow,
	}

	cfg, err := getCachedConfig(ctx, cfgOpts)
	if err != nil {
		return nil, errors.Wrap(err, "getting AWS config")
	}

	var s3Opts []func(*s3.Options)
	if p.AWSKey != "" {
		s3Opts = append(s3Opts, func(opts *s3.Options) {
			opts.Credentials = CreateAWSStaticCredentials(p.AWSKey, p.AWSSecret, p.AWSSessionToken)
		})
	} else if p.AWSRoleARN != "" {
		stsClient := sts.NewFromConfig(*cfg)
		s3Opts = append(s3Opts, func(opts *s3.Options) {
			opts.Credentials = CreateAWSAssumeRoleCredentials(stsClient, p.AWSRoleARN, p.ExternalID)
		})
	}

	return s3.NewFromConfig(*cfg, s3Opts...), nil
}

// PreSign returns a presigned URL that expires in 24 hours.
func PreSign(ctx context.Context, r PreSignRequestParams) (string, error) {
	svc, err := r.getS3Client(ctx)
	if err != nil {
		return "", errors.Wrap(err, "getting S3 client")
	}
	presignClient := s3.NewPresignClient(svc)

	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.Bucket),
		Key:    aws.String(r.FileKey),
	})
	if err != nil {
		return "", errors.Wrap(err, "pre-signing object")
	}

	return req.URL, nil
}

// GetHeadObject fetches the metadata of an S3 object. Warning: despite the
// input parameters, the function doesn't pre-sign the request to get the head
// object at all.
func GetHeadObject(ctx context.Context, r PreSignRequestParams) (*s3.HeadObjectOutput, error) {
	svc, err := r.getS3Client(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting S3 client")
	}

	return svc.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.Bucket),
		Key:    aws.String(r.FileKey),
	})
}
