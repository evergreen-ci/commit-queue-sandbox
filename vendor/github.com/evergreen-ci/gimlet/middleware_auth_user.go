package gimlet

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
)

// UserMiddlewareConfiguration is an keyed-arguments struct used to
// produce the user manager middleware.
type UserMiddlewareConfiguration struct {
	SkipCookie      bool
	SkipHeaderCheck bool
	HeaderUserName  string
	HeaderKeyName   string
	OIDC            *OIDCConfig
	CookieName      string
	CookiePath      string
	CookieTTL       time.Duration
	CookieDomain    string
}

// OIDCConfig configures the validation of JWTs provided as a header on requests.
type OIDCConfig struct {
	// HeaderName is the name of the header expected to contain the JWT.
	HeaderName string
	// Issuer is the expected issuer of the JWT.
	Issuer string
	// KeysetURL is a URL to download a remote keyset from to use for validating the JWT.
	KeysetURL string
	// DisplayNameFromID parses a display name from the subject in the JWT. If not provided the
	// display name will default to the token's subject.
	DisplayNameFromID func(string) string
}

func (o *OIDCConfig) validate() error {
	if o == nil {
		return nil
	}

	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(o.HeaderName == "", "header name must be provided")
	catcher.NewWhen(o.Issuer == "", "issuer must be provided")
	catcher.NewWhen(o.KeysetURL == "", "keyset URL must be provided")

	return catcher.Resolve()
}

// Validate ensures that the UserMiddlewareConfiguration is correct
// and internally consistent.
func (umc *UserMiddlewareConfiguration) Validate() error {
	catcher := grip.NewBasicCatcher()

	if !umc.SkipCookie {
		catcher.NewWhen(umc.CookieName == "", "must specify cookie name when cookie authentication is enabled")
		catcher.NewWhen(umc.CookieTTL < time.Second, "cookie timeout must be greater than or equal to a second")

		if umc.CookiePath == "" {
			umc.CookiePath = "/"
		} else if !strings.HasPrefix(umc.CookiePath, "/") {
			catcher.New("cookie path must begin with '/'")
		}
	}

	if !umc.SkipHeaderCheck {
		catcher.NewWhen(umc.HeaderUserName == "", "must specify a header user name when header auth is enabled")
		catcher.NewWhen(umc.HeaderKeyName == "", "must specify a header key name when header auth is enabled")
	}

	catcher.AddWhen(umc.OIDC != nil, umc.OIDC.validate())

	return catcher.Resolve()
}

// AttachCookie sets a cookie with the specified cookie to the
// request, according to the configuration of the user manager.
func (umc UserMiddlewareConfiguration) AttachCookie(token string, rw http.ResponseWriter) {
	http.SetCookie(rw, &http.Cookie{
		Name:     umc.CookieName,
		Path:     umc.CookiePath,
		Value:    token,
		HttpOnly: true,
		Expires:  time.Now().Add(umc.CookieTTL),
		Domain:   umc.CookieDomain,
	})
}

// ClearCookie removes the cookie defied in the user manager.
func (umc UserMiddlewareConfiguration) ClearCookie(rw http.ResponseWriter) {
	http.SetCookie(rw, &http.Cookie{
		Name:   umc.CookieName,
		Path:   umc.CookiePath,
		Domain: umc.CookieDomain,
		Value:  "",
		MaxAge: -1,
	})
}

func setUserForRequest(r *http.Request, u User) *http.Request {
	userID := u.Username()
	AddLoggingAnnotation(r, "user", userID)
	ctx := r.Context()
	ctx = utility.ContextWithAppendedAttributes(ctx, []attribute.KeyValue{
		attribute.String(userIDAttribute, userID),
	})
	ctx = AttachUser(ctx, u)
	return r.WithContext(ctx)
}

// AttachUser adds a user to a context. This function is public to
// support teasing workflows.
func AttachUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// GetUser returns the user attached to the request. The User object
// is nil when
func GetUser(ctx context.Context) User {
	u := ctx.Value(userKey)
	if u == nil {
		return nil
	}

	usr, ok := u.(User)
	if !ok {
		return nil
	}

	return usr
}

type userMiddleware struct {
	conf         UserMiddlewareConfiguration
	manager      UserManager
	oidcVerifier *oidc.IDTokenVerifier
}

// UserMiddleware produces a middleware that parses requests and uses
// the UserManager attached to the request to find and attach a user
// to the request.
func UserMiddleware(ctx context.Context, um UserManager, conf UserMiddlewareConfiguration) Middleware {
	middleware := &userMiddleware{
		conf:    conf,
		manager: um,
	}

	if conf.OIDC != nil {
		middleware.oidcVerifier = oidc.NewVerifier(
			conf.OIDC.Issuer,
			oidc.NewRemoteKeySet(ctx, conf.OIDC.KeysetURL),
			&oidc.Config{SkipClientIDCheck: true},
		)
	}

	return middleware
}

var ErrNeedsReauthentication = errors.New("user session has expired so they must be reauthenticated")

func (u *userMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) { //nolint: gocyclo
	var err error
	var usr User
	ctx := r.Context()
	reqID := GetRequestID(ctx)
	logger := GetLogger(ctx)

	if !u.conf.SkipCookie {
		var token string

		// Grab token auth from cookies
		for _, cookie := range r.Cookies() {
			if cookie.Name == u.conf.CookieName {
				if token, err = url.QueryUnescape(cookie.Value); err == nil {
					// set the user, preferring the cookie, maybe change
					if len(token) > 0 {
						usr, err = u.manager.GetUserByToken(ctx, token)
						needsReauth := errors.Cause(err) == ErrNeedsReauthentication

						logger.DebugWhen(err != nil && !needsReauth, message.WrapError(err, message.Fields{
							"request": reqID,
							"message": "problem getting user by token",
						}))
						if err == nil {
							usr, err = u.manager.GetOrCreateUser(usr)
							// Get the user's full details from the DB or create them if they don't exists
							if err != nil {
								logger.Debug(message.WrapError(err, message.Fields{
									"message": "error looking up user",
									"request": reqID,
								}))
							}
						}

						if usr != nil && !needsReauth {
							r = setUserForRequest(r, usr)
							break
						}
					}
				}
			}
		}
	}

	if !u.conf.SkipHeaderCheck {
		var (
			authDataAPIKey string
			authDataName   string
		)

		// Grab API auth details from header
		if len(r.Header[u.conf.HeaderKeyName]) > 0 {
			authDataAPIKey = r.Header[u.conf.HeaderKeyName][0]
		}
		if len(r.Header[u.conf.HeaderUserName]) > 0 {
			authDataName = r.Header[u.conf.HeaderUserName][0]
		}

		if len(authDataName) > 0 && len(authDataAPIKey) > 0 {
			usr, err = u.manager.GetUserByID(authDataName)
			logger.Debug(message.WrapError(err, message.Fields{
				"message":   "problem getting user by id",
				"operation": "header check",
				"name":      authDataName,
				"request":   reqID,
			}))

			// only loggable if the err is non-nil
			if err == nil && usr != nil {
				if usr.GetAPIKey() != authDataAPIKey {
					WriteTextResponse(rw, http.StatusUnauthorized, "invalid API key")
					return
				}
				r = setUserForRequest(r, usr)
			}
		}
	}

	if u.oidcVerifier != nil {
		if jwt := r.Header.Get(u.conf.OIDC.HeaderName); len(jwt) > 0 {
			usr, err := u.getUserForOIDCHeader(ctx, jwt)
			logger.DebugWhen(err != nil, message.WrapError(err, message.Fields{
				"message": "getting user for OIDC header",
				"request": reqID,
			}))
			if err == nil && usr != nil {
				r = setUserForRequest(r, usr)
			}
		}
	}

	next(rw, r)
}

const unauthorizedSpifeServiceUser = "istio-ingressgateway-public-service-account"
const spiffeRoute = "spiffe://cluster.local/ns/routing"

func (u *userMiddleware) getUserForOIDCHeader(ctx context.Context, header string) (User, error) {
	token, err := u.oidcVerifier.Verify(ctx, header)
	if err != nil {
		return nil, errors.Wrap(err, "verifying jwt")
	}

	claims := struct {
		Email string `json:"email"`
	}{}
	if err := token.Claims(&claims); err != nil {
		return nil, errors.Wrap(err, "parsing token claims")
	}
	// if the subject starts with the spiffe route, then
	// ignore it if it's the unauthorized user.
	if strings.HasPrefix(token.Subject, spiffeRoute) {
		if strings.HasSuffix(token.Subject, unauthorizedSpifeServiceUser) {
			return nil, nil
		}
	}

	displayName := token.Subject
	if u.conf.OIDC.DisplayNameFromID != nil {
		displayName = u.conf.OIDC.DisplayNameFromID(token.Subject)
	}

	usr, err := u.manager.GetOrCreateUser(NewBasicUser(BasicUserOptions{
		id:    token.Subject,
		name:  displayName,
		email: claims.Email,
	}))
	if err != nil {
		return nil, errors.Wrapf(err, "getting or creating user '%s'", displayName)
	}

	return usr, nil
}
