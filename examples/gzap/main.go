package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/iot-sonata/restful-contrib/gzap"
	"go.uber.org/zap"
)

func main() {
	ws := new(restful.WebService)

	logger, _ := zap.NewProduction()

	// Add a ginzap middleware, which:
	//   - Logs all requests, like a combined access and error log.
	//   - Logs to stdout.
	//   - RFC3339 with UTC time format.
	ws.Filter(gzap.Logger(logger,
		gzap.WithCustomFields(
			gzap.String("app", "example"),
			func(req *restful.Request, resp *restful.Response) zap.Field {

				return zap.String("custom field1", req.Request.URL.RawPath /*c.ClientIP()*/)
			},
			func(req *restful.Request, resp *restful.Response) zap.Field {
				return zap.String("custom field2", req.Request.URL.RawPath /*c.ClientIP()*/)
			},
		),
		gzap.WithSkipLogging(func(req *restful.Request, resp *restful.Response) bool {
			return req.Request.URL.Path == "/skiplogging"
		}),
		gzap.WithEnableBody(true),
	))

	// Logs all panic to error log
	//   - stack means whether output the stack info.
	ws.Filter(gzap.Recovery(logger, true,
		gzap.WithCustomFields(
			gzap.Any("app", "example"),
			func(req *restful.Request, resp *restful.Response) zap.Field {
				return zap.String("custom field1", req.Request.URL.RawPath /*c.ClientIP()*/)
			},
			func(req *restful.Request, resp *restful.Response) zap.Field {
				return zap.String("custom field2", req.Request.URL.RawPath /*c.ClientIP()*/)
			},
		),
	))

	// Example ping request.
	ws.Route(ws.GET("/ping/{id}").To(func(req *restful.Request, resp *restful.Response) {
		time.Sleep(time.Millisecond * 100)
		resp.WriteHeader(200)
		_, _ = io.WriteString(resp, "pong "+fmt.Sprint(time.Now().Unix()))
	}).Doc("ping一下"))

	// Example when panic happen.
	ws.Route(ws.GET("/panic").To(func(req *restful.Request, resp *restful.Response) {
		panic("An unexpected error happen!")
	}))

	ws.Route(ws.GET("/error").To(func(req *restful.Request, resp *restful.Response) {
		_ = resp.WriteError(http.StatusOK, errors.New("An error happen 1"))
	}))

	ws.Route(ws.GET("/skiplogging").To(func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
		_, _ = io.WriteString(resp, "i am skip logging, log should be not output")
	}))
	restful.Add(ws)
	// Listen and Server in 0.0.0.0:8080
	// DO NOT wrap http.ListenAndServe with log.Fatal in production
	// or you won't be able to drain in-flight request gracefully, even you handle sigterm
	log.Fatal(http.ListenAndServe(":8080", nil))

}
