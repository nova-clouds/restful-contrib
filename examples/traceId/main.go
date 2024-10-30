package main

import (
	"log"
	"net/http"

	"github.com/emicklei/go-restful/v3"
	"github.com/iot-labs-team/restful-contrib/traceid"
)

func main() {
	ws := new(restful.WebService)
	ws.Filter(traceid.TraceId())
	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		_, _ = resp.Write([]byte(traceid.FromTraceId(req.Request.Context())))
	}))
	restful.Add(ws)

	// DO NOT wrap http.ListenAndServe with log.Fatal in production
	// or you won't be able to drain in-flight request gracefully, even you handle sigterm
	log.Fatal(http.ListenAndServe(":8080", nil))
}
