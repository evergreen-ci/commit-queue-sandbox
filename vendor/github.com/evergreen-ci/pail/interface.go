package pail

import (
	"context"
	"io"
)

// Bucket defines an interface for accessing a remote blob store, like
// S3. Should be generic enough to be implemented for GCP equivalent.
//
// Other goals of this project are to allow us to have a single
// interface for interacting with blob storage, and allow us to fully
// move off of our legacy goamz package and stabalize all blob-storage
// operations across all projects. There should be no interface
// dependencies on external packages required to use this library.
//
// See, the following implemenations for previous approaches.
//
//   - https://github.com/evergreen-ci/evergreen/blob/master/thirdparty/s3.go
//   - https://github.com/mongodb/curator/tree/master/sthree
//
// The preferred AWS SDK is here: https://docs.aws.amazon.com/sdk-for-go/api/
//
// In no particular order:
//   - implementation constructors should make it possible to use
//     custom http.Clients (to aid in pooling.)
//   - We should probably implement .String methods.
//   - Do use the grip package for logging.
//   - get/put should support multipart upload/download?
//   - we'll want to do retries with back-off (potentially configurable
//     in bucketinfo?)
//   - we might need to have variants that Put/Get byte slices rather
//     than readers.
//   - pass contexts to requests for timeouts.
type Bucket interface {
	// Check validity of the bucket. This is dependent on the underlying
	// implementation.
	Check(context.Context) error

	// Exists returns whether the given key exists in the bucket or not.
	Exists(context.Context, string) (bool, error)

	// Join concatenates elements with the appropriate path separator of
	// the bucket, ignoring empty elements. This is analogous to
	// `filepath.Join`.
	Join(...string) string

	// Produces a Writer and Reader interface to the file named by
	// the string.
	Writer(context.Context, string) (io.WriteCloser, error)
	Reader(context.Context, string) (io.ReadCloser, error)

	// Put and Get write simple byte streams (in the form of
	// io.Readers) to/from specified keys.
	//
	// TODO: consider if these, particularly Get are not
	// substantively different from Writer/Reader methods, or
	// might just be a wrapper.
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)

	// Upload and Download write files from the local file
	// system to the specified key.
	Upload(context.Context, string, string) error
	Download(context.Context, string, string) error

	SyncBucket

	// Copy does a special copy operation that does not require downloading
	// a file. Note that CopyOptions.DestinationBucket must have the same
	// type as the calling bucket object.
	Copy(context.Context, CopyOptions) error

	// Remove the specified object(s) from the bucket.
	// RemoveMany continues on error and returns any accumulated errors.
	Remove(context.Context, string) error
	RemoveMany(context.Context, ...string) error

	// Remove all objects with the given prefix, continuing on error and
	// returning any accumulated errors.
	// Note that this operation is not atomic.
	RemovePrefix(context.Context, string) error

	// Remove all objects matching the given regular expression, continuing
	// on error and returning any accumulated errors.
	// Note that this operation is not atomic.
	RemoveMatching(context.Context, string) error

	// List returns an iterator over the contents of a bucket with the
	// the given prefix. Contents are iterated lexicographically by key
	// name.
	List(context.Context, string) (BucketIterator, error)

	// String returns the bucket name.
	String() string
	// MoveObjects moves multiple objects from sourceKeys in this bucket to destKeys in another bucket specified by destBucket.
	// The lengths of sourceKeys and destKeys must match.
	MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error
}

// FastGetS3Bucket is a Bucket but with an additional method, GetToWriter. Only S3
// bucket types can support this access pattern.
type FastGetS3Bucket interface {
	Bucket
	// GetToWriter allows the user to pass in an io.WriterAt (which is likely
	// going to be a file) that has the contents of the remote key automatically
	// copied to it in parallel. GetToWriter is significantly more efficient for
	// large files than a Bucket.Get.
	GetToWriter(context.Context, string, io.WriterAt) error
}

// SyncBucket defines an interface to access a remote blob store and synchronize
// the local file system tree with the remote store.
type SyncBucket interface {
	// Sync methods: these methods are the recursive, efficient
	// copy methods of files from S3 to the local file
	// system.
	Push(context.Context, SyncOptions) error
	Pull(context.Context, SyncOptions) error
}

// SyncOptions describes the arguments to the sync operations (Push and Pull).
// Note that exclude is a regular expression.
type SyncOptions struct {
	Local   string
	Remote  string
	Exclude string
}

// CopyOptions describes the arguments to the Copy method for moving
// objects between Buckets.
type CopyOptions struct {
	SourceKey         string
	DestinationKey    string
	DestinationBucket Bucket
	IsDestination     bool
}

////////////////////////////////////////////////////////////////////////
//
// Iterator
// While iterators (typically) use channels internally, this is a
// fairly standard paradigm for iterating through resources, and is
// use heavily in the FTDC library (https://github.com/mongodb/ftdc)
// and BSON (https://godoc.org/github.com/mongodb/mongo-go-driver/bson)
// libraries.

// BucketIterator provides a way to interact with the contents of a
// bucket, as in the output of the List operation.
type BucketIterator interface {
	Next(context.Context) bool
	Err() error
	Item() BucketItem
}

// BucketItem provides a basic interface for getting an object from a
// bucket.
type BucketItem interface {
	Bucket() string
	Name() string
	Hash() string
	Get(context.Context) (io.ReadCloser, error)
}

type bucketItemImpl struct {
	bucket string
	key    string
	hash   string

	// TODO add other info?

	// QUESTION: does this need to be an interface to support
	// additional information?

	b Bucket
}

func (bi *bucketItemImpl) Name() string   { return bi.key }
func (bi *bucketItemImpl) Hash() string   { return bi.hash }
func (bi *bucketItemImpl) Bucket() string { return bi.bucket }
func (bi *bucketItemImpl) Get(ctx context.Context) (io.ReadCloser, error) {
	return bi.b.Get(ctx, bi.key)
}
