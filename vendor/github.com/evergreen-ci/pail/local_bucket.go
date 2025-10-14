package pail

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type localFileSystem struct {
	path         string
	prefix       string
	useSlash     bool
	dryRun       bool
	deleteOnPush bool
	deleteOnPull bool
	verbose      bool
}

func (b *localFileSystem) String() string {
	return b.path
}

// LocalOptions describes the configuration of a local Bucket.
type LocalOptions struct {
	Path   string
	Prefix string
	// UseSlash sets the prefix separator to the slash ('/') character
	// instead of the OS specific separator.
	UseSlash     bool
	DryRun       bool
	DeleteOnSync bool
	DeleteOnPush bool
	DeleteOnPull bool
	Verbose      bool
}

func (o *LocalOptions) validate() error {
	if (o.DeleteOnPush != o.DeleteOnPull) && o.DeleteOnSync {
		return errors.New("ambiguous delete on sync options set")
	}

	return nil
}

func (b *localFileSystem) normalizeKey(key string) string {
	if key == "" {
		return b.prefix
	}
	return b.Join(b.prefix, key)
}

// NewLocalBucket returns an implementation of the Bucket interface
// that stores files in the local file system. Returns an error if the
// directory doesn't exist.
func NewLocalBucket(opts LocalOptions) (Bucket, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	b := &localFileSystem{
		path:         opts.Path,
		prefix:       opts.Prefix,
		useSlash:     opts.UseSlash,
		dryRun:       opts.DryRun,
		deleteOnPush: opts.DeleteOnPush || opts.DeleteOnSync,
		deleteOnPull: opts.DeleteOnPull || opts.DeleteOnSync,
	}
	if err := b.Check(context.TODO()); err != nil {
		return nil, errors.WithStack(err)
	}
	return b, nil
}

// NewLocalTemporaryBucket returns an "local" bucket implementation
// that stores resources in the local filesystem in a temporary
// directory created for this purpose. Returns an error if there were
// issues creating the temporary directory. This implementation does
// not provide a mechanism to delete the temporary directory.
func NewLocalTemporaryBucket(opts LocalOptions) (Bucket, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	dir, err := ioutil.TempDir("", "pail-local-tmp-bucket")
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary directory")
	}

	return &localFileSystem{
		path:         dir,
		prefix:       opts.Prefix,
		dryRun:       opts.DryRun,
		deleteOnPush: opts.DeleteOnPush || opts.DeleteOnSync,
		deleteOnPull: opts.DeleteOnPull || opts.DeleteOnSync,
	}, nil
}

func (b *localFileSystem) Check(_ context.Context) error {
	if _, err := os.Stat(b.path); os.IsNotExist(err) {
		return errors.New("bucket prefix does not exist")
	}

	return nil
}

func (b *localFileSystem) Exists(_ context.Context, key string) (bool, error) {
	if _, err := os.Stat(b.Join(b.path, b.normalizeKey(key))); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "getting file stats")
	}

	return true, nil
}

func (b *localFileSystem) Join(elems ...string) string {
	if b.useSlash {
		return consistentJoin(elems)
	}

	return filepath.Join(elems...)
}

func (b *localFileSystem) Writer(_ context.Context, name string) (io.WriteCloser, error) {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "writer",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           name,
	})

	if b.dryRun {
		return &mockWriteCloser{}, nil
	}

	path := b.Join(b.path, b.normalizeKey(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, errors.Wrap(err, "creating base directories")
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, errors.Wrapf(err, "opening file '%s'", path)
	}

	return f, nil
}

func (b *localFileSystem) Reader(_ context.Context, name string) (io.ReadCloser, error) {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"operation":     "reader",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           name,
	})

	path := b.Join(b.path, b.normalizeKey(name))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = MakeKeyNotFoundError(err)
		}
		return nil, errors.Wrapf(err, "opening file '%s'", path)
	}

	return f, nil
}

func (b *localFileSystem) Put(ctx context.Context, name string, input io.Reader) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "put",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           name,
	})

	f, err := b.Writer(ctx, name)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = io.Copy(f, input)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "copying data to file")
	}

	return errors.WithStack(f.Close())
}

func (b *localFileSystem) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"operation":     "get",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           name,
	})

	return b.Reader(ctx, name)
}

func (b *localFileSystem) Upload(ctx context.Context, name, path string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "upload",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
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

func (b *localFileSystem) Download(ctx context.Context, name, path string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"operation":     "download",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           name,
		"path":          path,
	})

	catcher := grip.NewBasicCatcher()

	if err := os.MkdirAll(filepath.Dir(path), 0600); err != nil {
		return errors.Wrapf(err, "creating enclosing directory for '%s'", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "creating file '%s'", path)
	}

	reader, err := b.Reader(ctx, name)
	if err != nil {
		_ = f.Close()
		return errors.WithStack(err)
	}

	_, err = io.Copy(f, reader)
	if err != nil {
		_ = f.Close()
		_ = reader.Close()
		return errors.Wrap(err, "copying data")
	}

	catcher.Add(reader.Close())
	catcher.Add(f.Close())
	return errors.WithStack(catcher.Resolve())
}

func (b *localFileSystem) Copy(ctx context.Context, options CopyOptions) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "copy",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"source_key":    options.SourceKey,
		"dest_key":      options.DestinationKey,
	})

	from, err := b.Reader(ctx, options.SourceKey)
	if err != nil {
		return errors.Wrap(err, "getting reader for source")
	}

	to, err := options.DestinationBucket.Writer(ctx, options.DestinationKey)
	if err != nil {
		return errors.Wrap(err, "getting writer for destination")
	}

	_, err = io.Copy(to, from)
	if err != nil {
		return errors.Wrap(err, "copying data")
	}

	return errors.WithStack(to.Close())
}

func (b *localFileSystem) Remove(ctx context.Context, key string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "remove",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"key":           key,
	})

	if b.dryRun {
		return nil
	}

	path := b.Join(b.path, b.normalizeKey(key))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		err = MakeKeyNotFoundError(err)
	}
	return errors.Wrapf(err, "removing path '%s'", path)
}

func (b *localFileSystem) RemoveMany(ctx context.Context, keys ...string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "remove many",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"keys":          keys,
	})

	catcher := grip.NewBasicCatcher()
	for _, key := range keys {
		catcher.Add(b.Remove(ctx, key))
	}
	return catcher.Resolve()
}

func (b *localFileSystem) RemovePrefix(ctx context.Context, prefix string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "remove prefix",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"prefix":        prefix,
	})

	return removePrefix(ctx, prefix, b)
}

func (b *localFileSystem) RemoveMatching(ctx context.Context, expression string) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "remove matching",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"expression":    expression,
	})

	return removeMatching(ctx, expression, b)
}

// MoveObjects moves multiple objects from sourceKeys in this local bucket to destKeys in another bucket.
// Not implemented for local buckets.
func (b *localFileSystem) MoveObjects(ctx context.Context, destBucket Bucket, sourceKeys, destKeys []string) error {
	return errors.New("MoveObjects is not implemented for local buckets")
}

func (b *localFileSystem) Push(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"dry_run":       b.dryRun,
		"operation":     "push",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
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

		target := b.Join(b.path, b.normalizeKey(b.Join(opts.Remote, fn)))
		file := b.Join(opts.Local, fn)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := b.Upload(ctx, b.Join(opts.Remote, fn), file); err != nil {
				return errors.WithStack(err)
			}

			continue
		}

		lsum, err := utility.SHA1SumFile(file)
		if err != nil {
			return errors.WithStack(err)
		}
		rsum, err := utility.SHA1SumFile(target)
		if err != nil {
			return errors.WithStack(err)
		}

		if lsum != rsum {
			if err := b.Upload(ctx, b.Join(opts.Remote, fn), file); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	if b.deleteOnPush && !b.dryRun {
		return errors.Wrap(deleteOnPush(ctx, files, opts.Remote, b), "deleting on sync after push")
	}
	return nil
}

func (b *localFileSystem) Pull(ctx context.Context, opts SyncOptions) error {
	grip.DebugWhen(b.verbose, message.Fields{
		"type":          "local",
		"operation":     "pull",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
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

	prefix := b.Join(b.path, b.normalizeKey(opts.Remote))
	files, err := walkLocalTree(ctx, prefix)
	if err != nil {
		return errors.WithStack(err)
	}

	keys := []string{}
	for _, fn := range files {
		if re != nil && re.MatchString(fn) {
			continue
		}

		keys = append(keys, fn)
		path := b.Join(opts.Local, fn)
		fn = b.Join(opts.Remote, fn)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := b.Download(ctx, fn, path); err != nil {
				return errors.WithStack(err)
			}

			continue
		}

		lsum, err := utility.SHA1SumFile(b.Join(prefix, fn))
		if err != nil {
			return errors.WithStack(err)
		}
		rsum, err := utility.SHA1SumFile(path)
		if err != nil {
			return errors.WithStack(err)
		}

		if lsum != rsum {
			if err := b.Download(ctx, fn, path); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	if b.deleteOnPull && !b.dryRun {
		return errors.Wrap(deleteOnPull(ctx, keys, opts.Local), "deleting on sync after pull")
	}
	return nil
}

func (b *localFileSystem) List(ctx context.Context, prefix string) (BucketIterator, error) {
	grip.DebugWhen(b.verbose, message.Fields{
		"operation":     "list",
		"bucket":        b.path,
		"bucket_prefix": b.prefix,
		"prefix":        prefix,
	})

	files, err := walkLocalTree(ctx, b.Join(b.path, b.normalizeKey(prefix)))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &localFileSystemIterator{
		files:  files,
		idx:    -1,
		bucket: b,
		prefix: prefix,
	}, nil
}

type localFileSystemIterator struct {
	err    error
	files  []string
	idx    int
	item   *bucketItemImpl
	bucket *localFileSystem
	prefix string
}

func (iter *localFileSystemIterator) Err() error       { return iter.err }
func (iter *localFileSystemIterator) Item() BucketItem { return iter.item }
func (iter *localFileSystemIterator) Next(_ context.Context) bool {
	iter.idx++
	if iter.idx > len(iter.files)-1 {
		return false
	}

	iter.item = &bucketItemImpl{
		bucket: iter.bucket.path,
		key:    iter.bucket.Join(iter.prefix, iter.files[iter.idx]),
		b:      iter.bucket,
	}
	return true
}
