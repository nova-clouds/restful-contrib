// Package gzap provides log handling using zap package.
package gzap

import (
	"bytes"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/emicklei/go-restful/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/iot-labs-team/restful-contrib/internal/pool"
)

// Option logger/recover option
type Option func(c *Config)

// WithCustomFields optional custom field
func WithCustomFields(fields ...func(req *restful.Request, resp *restful.Response) zap.Field) Option {
	return func(c *Config) {
		c.customFields = fields
	}
}

// WithSkipLogging optional custom skip logging option.
func WithSkipLogging(f func(req *restful.Request, resp *restful.Response) bool) Option {
	return func(c *Config) {
		if f != nil {
			c.skipLogging = f
		}
	}
}

// WithEnableBody optional custom enable request/response body.
func WithEnableBody(b bool) Option {
	return func(c *Config) {
		c.enableBody.Store(b)
	}
}

// WithExternalEnableBody optional custom enable request/response body control by external itself.
func WithExternalEnableBody(b *atomic.Bool) Option {
	return func(c *Config) {
		if b != nil {
			c.enableBody = b
		}
	}
}

// WithBodyLimit optional custom request/response body limit.
// default: <=0, mean not limit
func WithBodyLimit(limit int) Option {
	return func(c *Config) {
		c.limit = limit
	}
}

// WithSkipRequestBody optional custom skip request body logging option.
func WithSkipRequestBody(f func(req *restful.Request, resp *restful.Response) bool) Option {
	return func(c *Config) {
		if f != nil {
			c.skipRequestBody = f
		}
	}
}

// WithSkipResponseBody optional custom skip response body logging option.
func WithSkipResponseBody(f func(req *restful.Request, resp *restful.Response) bool) Option {
	return func(c *Config) {
		if f != nil {
			c.skipResponseBody = f
		}
	}
}

// WithUseLoggerLevel optional use logging level.
func WithUseLoggerLevel(f func(req *restful.Request, resp *restful.Response) zapcore.Level) Option {
	return func(c *Config) {
		if f != nil {
			c.useLoggerLevel = f
		}
	}
}

// Config logger/recover config
type Config struct {
	customFields []func(req *restful.Request, resp *restful.Response) zap.Field
	// if returns true, it will skip logging.
	skipLogging func(req *restful.Request, resp *restful.Response) bool
	// if returns true, it will skip request body.
	skipRequestBody func(req *restful.Request, resp *restful.Response) bool
	// if returns true, it will skip response body.
	skipResponseBody func(req *restful.Request, resp *restful.Response) bool
	// use logger level,
	// default:
	// 	zap.ErrorLevel: when status >= http.StatusInternalServerError && status <= http.StatusNetworkAuthenticationRequired
	// 	zap.WarnLevel: when status >= http.StatusBadRequest && status <= http.StatusUnavailableForLegalReasons
	//  zap.InfoLevel: otherwise.
	useLoggerLevel func(req *restful.Request, resp *restful.Response) zapcore.Level
	enableBody     *atomic.Bool // enable request/response body
	limit          int          // <=0: mean not limit
}

func skipRequestBody(req *restful.Request, resp *restful.Response) bool {
	v := req.Request.Header.Get("Content-Type")
	d, params, err := mime.ParseMediaType(v)
	if err != nil || !(d == "multipart/form-data" || d == "multipart/mixed") {
		return false
	}
	_, ok := params["boundary"]
	return ok
}

func skipResponseBody(req *restful.Request, resp *restful.Response) bool {
	// TODO: add skip response body rule
	return false
}

func useLoggerLevel(req *restful.Request, resp *restful.Response) zapcore.Level {
	status := resp.StatusCode()
	if status >= http.StatusInternalServerError &&
		status <= http.StatusNetworkAuthenticationRequired {
		return zap.ErrorLevel
	}
	if status >= http.StatusBadRequest &&
		status <= http.StatusUnavailableForLegalReasons &&
		status != http.StatusUnauthorized {
		return zap.WarnLevel
	}
	return zap.InfoLevel
}

func newConfig() Config {
	return Config{
		customFields:     nil,
		skipLogging:      func(req *restful.Request, resp *restful.Response) bool { return false },
		skipRequestBody:  func(req *restful.Request, resp *restful.Response) bool { return false },
		skipResponseBody: func(req *restful.Request, resp *restful.Response) bool { return false },
		useLoggerLevel:   useLoggerLevel,
		enableBody:       &atomic.Bool{},
		limit:            0,
	}
}

// Logger returns a gin.HandlerFunc (middleware) that logs requests using uber-go/zap.
//
// Requests with errors are logged using zap.Error().
// Requests without errors are logged using zap.Info().
func Logger(logger *zap.Logger, opts ...Option) restful.FilterFunction {
	cfg := newConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		respBodyBuilder := &strings.Builder{}
		reqBody := "skip request body"

		if cfg.enableBody.Load() {
			resp.ResponseWriter = &bodyWriter{ResponseWriter: resp.ResponseWriter, dupBody: respBodyBuilder}
			if hasSkipRequestBody := skipRequestBody(req, resp) || cfg.skipRequestBody(req, resp); !hasSkipRequestBody {
				reqBodyBuf, err := io.ReadAll(req.Request.Body)
				if err != nil {
					_ = resp.WriteError(http.StatusInternalServerError, err)
					return
				}
				req.Request.Body.Close()
				req.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBuf))
				if cfg.limit > 0 && len(reqBodyBuf) >= cfg.limit {
					reqBody = "larger request body"
				} else {
					reqBody = string(reqBodyBuf)
				}
			}
		}

		start := time.Now()
		// some evil middlewares modify this values
		path := req.Request.URL.Path
		query := req.Request.URL.RawQuery

		defer func() {
			if cfg.skipLogging(req, resp) {
				return
			}
			var level zapcore.Level

			if resp.Error() != nil {
				level = zapcore.ErrorLevel
			} else {
				level = cfg.useLoggerLevel(req, resp)
			}

			fc := pool.Get()
			defer pool.Put(fc)
			route := ""
			title := zap.Skip()
			if r := req.SelectedRoute(); r != nil {
				route = r.Path()
				title = zap.String("title", r.Doc())
			}
			fc.Fields = append(fc.Fields,
				title,
				zap.Int("status", resp.StatusCode()),
				zap.String("method", req.Request.Method),
				zap.String("path", path),
				zap.String("route", route),
				zap.String("query", query),
				zap.String("ip", req.Request.RemoteAddr),
				zap.String("user-agent", req.Request.UserAgent()),
				zap.Duration("latency", time.Since(start)),
			)
			if cfg.enableBody.Load() {
				respBody := "skip response body"
				if hasSkipResponseBody := skipResponseBody(req, resp) || cfg.skipResponseBody(req, resp); !hasSkipResponseBody {
					if cfg.limit > 0 && respBodyBuilder.Len() >= cfg.limit {
						respBody = "larger response body"
					} else {
						respBody = respBodyBuilder.String()
					}
				}
				fc.Fields = append(fc.Fields,
					zap.String("requestBody", reqBody),
					zap.String("responseBody", respBody),
				)
			}
			for _, fieldFunc := range cfg.customFields {
				fc.Fields = append(fc.Fields, fieldFunc(req, resp))
			}
			if err := resp.Error(); err != nil {
				fc.Fields = append(fc.Fields, zap.Error(err))
			}
			logger.Log(level, "logging", fc.Fields...)
		}()
		chain.ProcessFilter(req, resp)
	}
}

// Recovery returns a gin.HandlerFunc (middleware)
// that recovers from any panics and logs requests using uber-go/zap.
// All errors are logged using zap.Error().
// stack means whether output the stack info.
// The stack info is easy to find where the error occurs but the stack info is too large.
func Recovery(logger *zap.Logger, stack bool, opts ...Option) restful.FilterFunction {
	cfg := newConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if stack {
		cfg.customFields = append(cfg.customFields, func(req *restful.Request, resp *restful.Response) zap.Field {
			return zap.ByteString("stack", debug.Stack())
		})
	}
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(req.Request, false)
				if brokenPipe {
					logger.Error(req.Request.URL.Path,
						zap.Any("error", err),
						zap.ByteString("request", httpRequest),
					)
					// If the connection is dead, we can't write a status to it.
					resp.WriteError(http.StatusInternalServerError, err.(error))
					return
				}

				fc := pool.Get()
				defer pool.Put(fc)
				fc.Fields = append(fc.Fields,
					zap.Any("error", err),
					zap.ByteString("request", httpRequest),
				)
				for _, field := range cfg.customFields {
					fc.Fields = append(fc.Fields, field(req, resp))
				}
				logger.Error("recovery from panic", fc.Fields...)
				resp.WriteErrorString(http.StatusInternalServerError, "panic")
			}
		}()
		chain.ProcessFilter(req, resp)
	}
}

type bodyWriter struct {
	http.ResponseWriter
	dupBody *strings.Builder
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	w.dupBody.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyWriter) WriteString(s string) (int, error) {
	w.dupBody.WriteString(s)
	return io.WriteString(w.ResponseWriter, s)
}

// Any custom immutable any field
func Any(key string, value any) func(req *restful.Request, resp *restful.Response) zap.Field {
	field := zap.Any(key, value)
	return func(req *restful.Request, resp *restful.Response) zap.Field { return field }
}

// String custom immutable string field
func String(key, value string) func(req *restful.Request, resp *restful.Response) zap.Field {
	field := zap.String(key, value)
	return func(req *restful.Request, resp *restful.Response) zap.Field { return field }
}

// Int64 custom immutable int64 field
func Int64(key string, value int64) func(req *restful.Request, resp *restful.Response) zap.Field {
	field := zap.Int64(key, value)
	return func(req *restful.Request, resp *restful.Response) zap.Field { return field }
}

// Uint64 custom immutable uint64 field
func Uint64(key string, value uint64) func(req *restful.Request, resp *restful.Response) zap.Field {
	field := zap.Uint64(key, value)
	return func(req *restful.Request, resp *restful.Response) zap.Field { return field }
}

// Float64 custom immutable float32 field
func Float64(key string, value float64) func(req *restful.Request, resp *restful.Response) zap.Field {
	field := zap.Float64(key, value)
	return func(req *restful.Request, resp *restful.Response) zap.Field { return field }
}
