package gimlet

import (
	"context"
	"net/http"
	"time"

	"github.com/evergreen-ci/negroni"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"go.opentelemetry.io/otel/attribute"
)

const (
	remoteAddrHeaderName = "X-Forwarded-For"
	requestIDAttribute   = "gimlet.request.id"
	userIDAttribute      = "gimlet.user.id"
)

// appLogging provides a Negroni-compatible middleware to send all
// logging using the grip packages logging. This defaults to using
// systemd logging, but gracefully falls back to use go standard
// library logging, with some additional helpers and configurations to
// support configurable level-based logging. This particular
// middlewear resembles the basic tracking provided by Negroni's
// standard logging system.
type appLogging struct {
	grip.Journaler
}

// NewAppLogger creates an logging middlear instance suitable for use
// with Negroni. Sets the logging configuration to be the same as the
// default global grip logging object.
func NewAppLogger() Middleware { return &appLogging{logging.MakeGrip(grip.GetSender())} }

func setServiceLogger(r *http.Request, logger grip.Journaler) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), loggerKey, logger))
}

type loggingAnnotations map[string]any

// AddLoggingAnnotation adds a key-value pair to be added to logging
// messages used by the application logging information, overwriting
// any previous value for the key.
// This will noop if the logger hasn't already been set up.
func AddLoggingAnnotation(r *http.Request, key string, value any) {
	annotations := getLoggingAnnotations(r.Context())
	if annotations == nil {
		return
	}

	annotations[key] = value
}

func setLoggingAnnotations(r *http.Request, annotations loggingAnnotations) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), loggingAnnotationsKey, annotations))
}

func getLoggingAnnotations(ctx context.Context) loggingAnnotations {
	rv := ctx.Value(loggingAnnotationsKey)
	if rv == nil {
		return nil
	}
	a, ok := rv.(loggingAnnotations)
	if !ok {
		return nil
	}

	return a
}

func setStartAtTime(r *http.Request, startAt time.Time) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), startAtKey, startAt))
}

func getRequestStartAt(ctx context.Context) time.Time {
	if rv := ctx.Value(startAtKey); rv != nil {
		if t, ok := rv.(time.Time); ok {
			return t
		}
	}

	return time.Time{}
}

// GetLogger produces a special logger attached to the request. If no
// request is attached, GetLogger returns a logger instance wrapping
// the global sender.
func GetLogger(ctx context.Context) grip.Journaler {
	if rv := ctx.Value(loggerKey); rv != nil {
		if l, ok := rv.(grip.Journaler); ok {
			return l
		}
	}

	return logging.MakeGrip(grip.GetSender())
}

// Logs the request path, the beginning of every request as well as
// the duration upon completion and the status of the response.
func (l *appLogging) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	r = setupLogger(l.Journaler, r)

	next(rw, r)
	res := rw.(negroni.ResponseWriter)
	finishLogger(l.Journaler, r, res)
}

func setupLogger(logger grip.Journaler, r *http.Request) *http.Request {
	r = setServiceLogger(r, logger)
	remote := r.Header.Get(remoteAddrHeaderName)
	if remote != "" {
		r.RemoteAddr = remote
	}

	id := getNumber()
	r = setRequestID(r, id)
	r = r.WithContext(utility.ContextWithAppendedAttributes(r.Context(), []attribute.KeyValue{attribute.Int(requestIDAttribute, id)}))

	startAt := time.Now()
	r = setStartAtTime(r, startAt)
	r = setLoggingAnnotations(r, loggingAnnotations{})

	return r
}

func finishLogger(logger grip.Journaler, r *http.Request, res negroni.ResponseWriter) {
	ctx := r.Context()
	startAt := getRequestStartAt(ctx)
	dur := time.Since(startAt)
	m := message.Fields{
		"action":      "completed",
		"method":      r.Method,
		"remote":      r.RemoteAddr,
		"request":     GetRequestID(ctx),
		"path":        r.URL.Path,
		"duration_ms": int64(dur / time.Millisecond),
		"status":      res.Status(),
	}
	if len(r.URL.Query()) > 0 {
		m["params"] = r.URL.Query()
	}

	a := getLoggingAnnotations(ctx)
	if a != nil {
		for key, val := range a {
			m[key] = val
		}
	}

	logger.Info(m)
}

// This is largely duplicated from the above, but lets us optionally
type appRecoveryLogger struct {
	grip.Journaler
}

// NewRecoveryLogger logs request start, end, and recovers from panics
// (logging the panic as well).
func NewRecoveryLogger(j grip.Journaler) Middleware { return &appRecoveryLogger{Journaler: j} }

// MakeRecoveryLoger constructs a middleware layer that logs request
// start, end, and recovers from panics (logging the panic as well).
//
// This logger uses the default grip logger.
func MakeRecoveryLogger() Middleware {
	return &appRecoveryLogger{Journaler: logging.MakeGrip(grip.GetSender())}
}

func (l *appRecoveryLogger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	r = setupLogger(l.Journaler, r)
	ctx := r.Context()

	defer func() {
		if err := recover(); err != nil {
			if rw.Header().Get("Content-Type") == "" {
				rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			}
			rw.WriteHeader(http.StatusInternalServerError)

			// handlers panic with http.ErrAbortHandler to abort and suppress logging
			// (https://golang.org/pkg/net/http/#Handler)
			// log at a lower level
			if err == http.ErrAbortHandler {
				l.Debug(message.Fields{
					"message":  "hit suppressed abort panic",
					"action":   "aborted",
					"request":  GetRequestID(ctx),
					"duration": time.Since(getRequestStartAt(ctx)),
					"path":     r.URL.Path,
					"remote":   r.RemoteAddr,
					"length":   r.ContentLength,
				})
			} else {
				_ = recovery.SendMessageWithPanicError(err, nil, l.Journaler, message.Fields{
					"action":   "aborted",
					"request":  GetRequestID(ctx),
					"duration": time.Since(getRequestStartAt(ctx)),
					"path":     r.URL.Path,
					"remote":   r.RemoteAddr,
					"length":   r.ContentLength,
				})
			}

			WriteJSONInternalError(rw, ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "request aborted",
			})
		}
	}()
	next(rw, r)

	res := rw.(negroni.ResponseWriter)
	finishLogger(l.Journaler, r, res)
}
