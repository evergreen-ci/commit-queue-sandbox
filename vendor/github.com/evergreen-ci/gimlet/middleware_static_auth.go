package gimlet

import (
	"net/http"
	"path"
	"strings"
)

// StaticAuth is a middleware handler that serves static files in the given
// directory/filesystem if the user associated with the request is authenticated.
// If the file does not exist on the filesystem, it  passes along to the next
// middleware in the chain. It is copied directly from negroni's StaticAuth,
// except for the last part that involves authenticating users.
type StaticAuth struct {
	// Dir is the directory to serve static files from
	Dir http.FileSystem
	// Prefix is the optional prefix used to serve the static directory content
	Prefix string
	// IndexFile defines which file to serve as index if it exists.
	IndexFile string
}

// NewStaticAuth provides a wrapper around negroni's static file
// server middleware with additional authentication handling.
func NewStaticAuth(prefix string, fs http.FileSystem) *StaticAuth {
	return &StaticAuth{
		Dir:       fs,
		Prefix:    prefix,
		IndexFile: "index.html",
	}
}

// ServeHTTP is the same logic as negroni's Static middleware handler, with the exception that it
// checks that a user is signed in and authenticated before serving the static content. It does this
// after validating that the file in the URL path points to a real file to prevent auth checking for
// files that do not exist.
func (s *StaticAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.Method != "GET" && r.Method != "HEAD" {
		next(rw, r)
		return
	}
	file := r.URL.Path
	// if we have a prefix, filter requests by stripping the prefix
	if s.Prefix != "" {
		if !strings.HasPrefix(file, s.Prefix) {
			next(rw, r)
			return
		}
		file = file[len(s.Prefix):]
		if file != "" && file[0] != '/' {
			next(rw, r)
			return
		}
	}
	f, err := s.Dir.Open(file)
	if err != nil {
		// discard the error?
		next(rw, r)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		next(rw, r)
		return
	}

	// try to serve index file
	if fi.IsDir() {
		// redirect if missing trailing slash
		if !strings.HasSuffix(r.URL.Path, "/") {
			if strings.HasPrefix(r.URL.Path, "//") {
				r.URL.Path = "/" + strings.TrimLeft(r.URL.Path, "/")
			}
			http.Redirect(rw, r, r.URL.Path+"/", http.StatusFound)
			return
		}

		file = path.Join(file, s.IndexFile)
		f, err = s.Dir.Open(file)
		if err != nil {
			next(rw, r)
			return
		}
		defer f.Close()

		fi, err = f.Stat()
		if err != nil || fi.IsDir() {
			next(rw, r)
			return
		}
	}

	ctx := r.Context()
	authenticator := GetAuthenticator(ctx)
	if authenticator == nil {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	user := GetUser(ctx)
	if user == nil {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !authenticator.CheckAuthenticated(user) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.ServeContent(rw, r, file, fi.ModTime(), f)
}
