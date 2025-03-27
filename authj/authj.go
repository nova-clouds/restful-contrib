package authj

import (
	"context"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/emicklei/go-restful/v3"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer, so it fits in an interface{} without allocation.
type ctxAuthKey struct{}

// Config for Authorizer
type Config struct {
	errFallback        func(*restful.Request, *restful.Response, error)
	forbiddenFallback  func(*restful.Request, *restful.Response)
	skipAuthentication func(*restful.Request, *restful.Response) bool
	subject            func(*restful.Request, *restful.Response) string
}

// Option config option
type Option func(*Config)

// WithErrorFallback set the fallback handler when request are error happened.
// default: the 500 server error to the client
func WithErrorFallback(fn func(*restful.Request, *restful.Response, error)) Option {
	return func(cfg *Config) {
		if fn != nil {
			cfg.errFallback = fn
		}
	}
}

// WithForbiddenFallback set the fallback handler when request are not allow.
// default: the 403 Forbidden to the client
func WithForbiddenFallback(fn func(*restful.Request, *restful.Response)) Option {
	return func(cfg *Config) {
		if fn != nil {
			cfg.forbiddenFallback = fn
		}
	}
}

// WithSubject set the subject extractor of the requests.
// default: Subject
func WithSubject(fn func(*restful.Request, *restful.Response) string) Option {
	return func(cfg *Config) {
		if fn != nil {
			cfg.subject = fn
		}
	}
}

// WithSkipAuthentication set the skip approve when it is return true.
// Default: always false
func WithSkipAuthentication(fn func(*restful.Request, *restful.Response) bool) Option {
	return func(cfg *Config) {
		if fn != nil {
			cfg.skipAuthentication = fn
		}
	}
}

// Authorizer returns the authorizer
// uses a Casbin enforcer, and Subject as subject.
func Authorizer(e casbin.IEnforcer, opts ...Option) restful.FilterFunction {
	cfg := Config{
		func(req *restful.Request, resp *restful.Response, err error) {
			resp.WriteHeaderAndJson( // nolint: errcheck
				http.StatusInternalServerError, map[string]any{
					"code": http.StatusInternalServerError,
					"msg":  "Permission validation errors occur!",
				},
				restful.MIME_JSON,
			)
		},
		func(req *restful.Request, resp *restful.Response) {
			resp.WriteHeaderAndJson( // nolint: errcheck
				http.StatusForbidden, map[string]any{
					"code": http.StatusForbidden,
					"msg":  "Permission denied!",
				},
				restful.MIME_JSON,
			)
		},
		func(req *restful.Request, resp *restful.Response) bool { return false },
		Subject,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		if !cfg.skipAuthentication(req, resp) {
			// checks the subject,path,method permission combination from the request.
			allowed, err := e.Enforce(cfg.subject(req, resp), req.Request.URL.Path, req.Request.Method)
			if err != nil {
				cfg.errFallback(req, resp, err)
				return
			}
			if !allowed {
				cfg.forbiddenFallback(req, resp)
				return
			}
		}
		chain.ProcessFilter(req, resp)
	}
}

// Subject returns the value associated with this context for subjectCtxKey,
func Subject(req *restful.Request, resp *restful.Response) string {
	val, _ := req.Request.Context().Value(ctxAuthKey{}).(string)
	return val
}

// ContextWithSubject return a copy of parent in which the value associated with
// subjectCtxKey is subject.
func ContextWithSubject(req *restful.Request, resp *restful.Response, subject string) {
	ctx := context.WithValue(req.Request.Context(), ctxAuthKey{}, subject)
	req.Request = req.Request.WithContext(ctx)
}
