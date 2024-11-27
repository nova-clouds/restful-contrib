package traceid

import (
	"context"

	"github.com/emicklei/go-restful/v3"
	"github.com/iot-sonata/restful-contrib/utilities/sequence"
)

// Key to use when setting the trace id.
type ctxTraceIdKey struct{}

// Config defines the config for TraceId middleware
type Config struct {
	traceIdHeader string
	nextTraceId   func() string
}

// Option TraceId option
type Option func(*Config)

// WithTraceIdHeader optional request id header (default "X-Trace-Id")
func WithTraceIdHeader(s string) Option {
	return func(c *Config) {
		c.traceIdHeader = s
	}
}

// WithNextTraceId optional next trace id function (default NewSequence function use utilities/sequence)
func WithNextTraceId(f func() string) Option {
	return func(c *Config) {
		c.nextTraceId = f
	}
}

// TraceId is a middleware that injects a trace id into the context of each
// request. if it is empty, set to write head
//   - traceIdHeader is the name of the HTTP Header which contains the trace id.
//     Exported so that it can be changed by developers. (default "X-Trace-Id")
//   - nextTraceId generates the next trace id.(default NewSequence function use utilities/sequence)
func TraceId(opts ...Option) restful.FilterFunction {
	cc := &Config{
		traceIdHeader: "X-Trace-Id",
		nextTraceId:   NextTraceId,
	}
	for _, opt := range opts {
		opt(cc)
	}
	return func(req *restful.Request, resp *restful.Response, fc *restful.FilterChain) {
		traceId := req.Request.Header.Get(cc.traceIdHeader)
		if traceId == "" {
			traceId = cc.nextTraceId()
		}
		// set response header
		resp.ResponseWriter.Header().Set(cc.traceIdHeader, traceId)
		// set request context
		req.Request = req.Request.WithContext(WithTraceId(req.Request.Context(), traceId))
		fc.ProcessFilter(req, resp)
	}
}

// WithTraceId Inject traceId to context.
func WithTraceId(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, ctxTraceIdKey{}, traceId)
}

// FromTraceId returns a trace id from the given context if one is present.
// Returns the empty string if a trace id cannot be found.
func FromTraceId(ctx context.Context) string {
	traceId, _ := ctx.Value(ctxTraceIdKey{}).(string)
	return traceId
}

// NextTraceId returns the next trace id, use sequence global sequence.
func NextTraceId() string {
	return sequence.NextSequence()
}
