package authorize

import (
	"io"
	"net/http"

	"github.com/emicklei/go-restful/v3"
)

// Option is Middleware option.
type Option func(*options)

// options is a Middleware option
type options struct {
	skip                 func(req *restful.Request, resp *restful.Response) bool
	unauthorizedFallback func(req *restful.Request, resp *restful.Response, err error)
}

// WithSkip set skip func
func WithSkip(f func(req *restful.Request, resp *restful.Response) bool) Option {
	return func(o *options) {
		if f != nil {
			o.skip = f
		}
	}
}

// WithUnauthorizedFallback sets the fallback handler when requests are unauthorized.
func WithUnauthorizedFallback(f func(req *restful.Request, resp *restful.Response, err error)) Option {
	return func(o *options) {
		if f != nil {
			o.unauthorizedFallback = f
		}
	}
}

func (a *Auth[T]) Middleware(opts ...Option) restful.FilterFunction {
	o := &options{
		unauthorizedFallback: func(req *restful.Request, resp *restful.Response, err error) {
			resp.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(resp, err.Error())
		},
		skip: func(req *restful.Request, resp *restful.Response) bool { return false },
	}
	for _, opt := range opts {
		opt(o)
	}
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		if !o.skip(req, resp) {
			acc, err := a.ParseFromRequest(req.Request)
			if err != nil {
				o.unauthorizedFallback(req, resp, err)
				return
			}
			req.Request = req.Request.WithContext(NewContext(req.Request.Context(), acc))
		}
		chain.ProcessFilter(req, resp)
	}

}
