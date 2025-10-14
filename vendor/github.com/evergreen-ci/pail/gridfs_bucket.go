package pail

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GridFSOptions support the use and creation of GridFS backed buckets.
type GridFSOptions struct {
	Name         string
	Prefix       string
	Database     string
	MongoDBURI   string
	DryRun       bool
	DeleteOnSync bool
	DeleteOnPush bool
	DeleteOnPull bool
	Verbose      bool
}

func (o *GridFSOptions) validate() error {
	if (o.DeleteOnPush != o.DeleteOnPull) && o.DeleteOnSync {
		return errors.New("ambiguous delete on sync options set")
	}

	return nil
}

type gridfsBucket struct {
	opts   GridFSOptions
	client *mongo.Client
}

func (b *gridfsBucket) String() string {
	return b.opts.Name
}

func (b *gridfsBucket) normalizeKey(key string) string { return b.Join(b.opts.Prefix, key) }

func (b *gridfsBucket) denormalizeKey(key string) string {
	return consistentTrimPrefix(key, b.opts.Prefix)
}

// NewGridFSBucketWithClient returns a new bucket backed by GridFS with the
// existing Mongo client and given options.
func NewGridFSBucketWithClient(ctx context.Context, client *mongo.Client, opts GridFSOptions) (Bucket, error) {
	if client == nil {
		return nil, errors.New("must provide a Mongo client")
	}

	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &gridfsBucket{opts: opts, client: client}, nil
}

// NewGridFSBucket returns a bucket backed by GridFS with the given options.
func NewGridFSBucket(ctx context.Context, opts GridFSOptions) (Bucket, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(opts.MongoDBURI))
	if err != nil {
		return nil, errors.Wrap(err, "constructing client")
	}

	return &gridfsBucket{opts: opts, client: client}, nil
}

func (b *gridfsBucket) Check(ctx context.Context) error {
	return errors.Wrap(b.client.Ping(ctx, nil), "pinging DB")
}

func (b *gridfsBucket) Exists(ctx context.Context, key string) (bool, error) {
	grid, err := b.bucket(ctx)
	if err != nil {
		return false, errors.Wrap(err, "resolving bucket")
	}

	if err = grid.GetFilesCollection().FindOne(ctx, bson.M{"filename": b.normalizeKey(key)}).Err(); err == mongo.ErrNoDocuments {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "finding file")
	}

	return true, nil
}

func (b *gridfsBucket) Join(elems ...string) string { return consistentJoin(elems) }

// bucket returns a new GridFS bucket configured with the context timeout, if
// it exists. This function is called by each operation that needs to read or
// write from the bucket to avoid read and write deadline conflicts—GridFS only
// supports a single read and a single write deadline for a bucket instance.
// This function sets the read and write deadlines to allow it to respect the
// context timeouts passed in by the caller.
func (b *gridfsBucket) bucket(ctx context.Context) (*gridfs.Bucket, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "fetching bucket with canceled context")
	}

	gfs, err := gridfs.NewBucket(b.client.Database(b.opts.Database), options.GridFSBucket().SetName(b.opts.Name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// The streaming GridFS functions do not accept a context so we need to
	// set the deadline from the passed-in context here, if it exists, in
	// order to respect context timeouts passed in by the caller.
	dl, ok := ctx.Deadline()
	if ok {
		_ = gfs.SetReadDeadline(dl)
		_ = gfs.SetWriteDeadline(dl)
	}

	return gfs, nil
}

func (b *gridfsBucket) Writer(ctx context.Context, name string) (io.WriteCloser, error) {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "writer",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
	})

	grid, err := b.bucket(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolving bucket")
	}

	if b.opts.DryRun {
		return &mockWriteCloser{}, nil
	}

	writer, err := grid.OpenUploadStream(b.normalizeKey(name))
	if err != nil {
		return nil, errors.Wrap(err, "opening stream")
	}

	return writer, nil
}

func (b *gridfsBucket) Reader(ctx context.Context, name string) (io.ReadCloser, error) {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "reader",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
	})

	grid, err := b.bucket(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolving bucket")
	}

	reader, err := grid.OpenDownloadStreamByName(b.normalizeKey(name))
	if err != nil {
		if err == gridfs.ErrFileNotFound {
			err = MakeKeyNotFoundError(err)
		}
		return nil, errors.Wrap(err, "opening stream")
	}

	return reader, nil
}

func (b *gridfsBucket) Put(ctx context.Context, name string, input io.Reader) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "put",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
	})

	grid, err := b.bucket(ctx)
	if err != nil {
		return errors.Wrap(err, "resolving bucket")
	}

	if b.opts.DryRun {
		return nil
	}

	if _, err = grid.UploadFromStream(b.normalizeKey(name), input); err != nil {
		return errors.Wrap(err, "uploading file")
	}

	return nil
}

func (b *gridfsBucket) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "get",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
	})

	return b.Reader(ctx, name)
}

func (b *gridfsBucket) Upload(ctx context.Context, name, path string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "upload",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
		"path":          path,
	})

	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "opening file '%s'", name)
	}
	defer f.Close()

	return errors.WithStack(b.Put(ctx, name, f))
}

func (b *gridfsBucket) Download(ctx context.Context, name, path string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "download",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           name,
		"path":          path,
	})

	reader, err := b.Reader(ctx, name)
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
	defer f.Close()

	_, err = io.Copy(f, reader)
	return errors.Wrap(err, "copying data to file")
}

func (b *gridfsBucket) Push(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "push",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
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

	localPaths, err := walkLocalTree(ctx, opts.Local)
	if err != nil {
		return errors.Wrap(err, "finding local paths")
	}

	for _, path := range localPaths {
		if re != nil && re.MatchString(path) {
			continue
		}

		target := b.Join(opts.Remote, path)
		if err = b.Upload(ctx, target, filepath.Join(opts.Local, path)); err != nil {
			return errors.Wrapf(err, "uploading file '%s' to '%s'", path, target)
		}
	}

	if (b.opts.DeleteOnPush || b.opts.DeleteOnSync) && !b.opts.DryRun {
		return errors.Wrap(deleteOnPush(ctx, localPaths, opts.Remote, b), "deleting on sync after push")
	}

	return nil
}

func (b *gridfsBucket) Pull(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "pull",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
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
		item := iter.Item()
		if re != nil && re.MatchString(item.Name()) {
			continue
		}

		localName, err := filepath.Rel(opts.Remote, item.Name())
		if err != nil {
			return errors.Wrap(err, "getting relative filepath")
		}
		keys = append(keys, localName)

		if err = b.Download(ctx, item.Name(), filepath.Join(opts.Local, localName)); err != nil {
			return errors.WithStack(err)
		}
	}

	if err = iter.Err(); err != nil {
		return errors.WithStack(err)
	}

	if (b.opts.DeleteOnPull || b.opts.DeleteOnSync) && !b.opts.DryRun {
		return errors.Wrap(deleteOnPull(ctx, keys, opts.Local), "deleting on sync after pull")
	}

	return nil
}

func (b *gridfsBucket) Copy(ctx context.Context, opts CopyOptions) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "copy",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"source_key":    opts.SourceKey,
		"dest_key":      opts.DestinationKey,
	})

	from, err := b.Reader(ctx, opts.SourceKey)
	if err != nil {
		return errors.Wrap(err, "getting reader for source")
	}

	to, err := opts.DestinationBucket.Writer(ctx, opts.DestinationKey)
	if err != nil {
		return errors.Wrap(err, "getting writer for destination")
	}

	if _, err = io.Copy(to, from); err != nil {
		return errors.Wrap(err, "copying data")
	}

	return errors.WithStack(to.Close())
}

func (b *gridfsBucket) Remove(ctx context.Context, key string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "remove",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"key":           key,
	})

	return b.RemoveMany(ctx, key)
}

func (b *gridfsBucket) RemoveMany(ctx context.Context, keys ...string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "remove many",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"keys":          keys,
	})

	grid, err := b.bucket(ctx)
	if err != nil {
		return errors.Wrap(err, "resolving bucket")
	}

	normalizedKeys := make([]string, len(keys))
	for i, key := range keys {
		normalizedKeys[i] = b.normalizeKey(key)
	}

	cur, err := grid.Find(bson.M{"filename": bson.M{"$in": normalizedKeys}})
	if err != nil {
		return errors.Wrap(err, "finding file(s)")
	}

	catcher := grip.NewBasicCatcher()
	document := struct {
		ID interface{} `bson:"_id"`
	}{}
	for cur.Next(ctx) {
		if err = cur.Decode(&document); err != nil {
			return errors.Wrap(err, "decoding GridFS metadata")
		}

		if b.opts.DryRun {
			continue
		}

		if err = grid.Delete(document.ID); err != nil {
			catcher.Wrap(err, "deleting GridFS file")
			break
		}
	}
	catcher.Wrap(cur.Err(), "iterating GridFS metadata")
	catcher.Wrap(cur.Close(ctx), "closing cursor")

	return catcher.Resolve()
}

func (b *gridfsBucket) RemovePrefix(ctx context.Context, prefix string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "remove prefix",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"prefix":        prefix,
	})

	return removePrefix(ctx, prefix, b)
}

func (b *gridfsBucket) RemoveMatching(ctx context.Context, expr string) error {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"dry_run":       b.opts.DryRun,
		"operation":     "remove matching",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"expression":    expr,
	})

	return removeMatching(ctx, expr, b)
}

func (b *gridfsBucket) List(ctx context.Context, prefix string) (BucketIterator, error) {
	grip.DebugWhen(b.opts.Verbose, message.Fields{
		"type":          "gridfs",
		"operation":     "list",
		"bucket":        b.opts.Name,
		"bucket_prefix": b.opts.Prefix,
		"prefix":        prefix,
	})

	grid, err := b.bucket(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolving bucket")
	}

	filter := bson.M{}
	if prefix != "" {
		filter = bson.M{"filename": primitive.Regex{Pattern: fmt.Sprintf("^%s.*", b.normalizeKey(prefix))}}
	}
	cursor, err := grid.Find(filter, options.GridFSFind().SetSort(bson.M{"filename": 1}))
	if err != nil {
		return nil, errors.Wrap(err, "finding file")
	}

	return &gridfsIterator{bucket: b, iter: cursor}, nil
}

type gridfsIterator struct {
	err    error
	bucket *gridfsBucket
	iter   *mongo.Cursor
	item   *bucketItemImpl
}

func (iter *gridfsIterator) Err() error       { return iter.err }
func (iter *gridfsIterator) Item() BucketItem { return iter.item }
func (iter *gridfsIterator) Next(ctx context.Context) bool {
	if !iter.iter.Next(ctx) {
		iter.err = iter.iter.Err()
		return false
	}

	document := struct {
		ID       interface{} `bson:"_id"`
		Filename string      `bson:"filename"`
	}{}
	if err := iter.iter.Decode(&document); err != nil {
		iter.err = err
		return false
	}

	iter.item = &bucketItemImpl{
		bucket: iter.bucket.opts.Name,
		key:    iter.bucket.denormalizeKey(document.Filename),
		b:      iter.bucket,
	}
	return true
}

func (b *gridfsBucket) MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error {
	return errors.New("MoveObjects is not implemented for GridFS buckets")
}
