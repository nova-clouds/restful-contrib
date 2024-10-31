package pprof

import (
	"net/http"
	"net/http/pprof"

	"github.com/emicklei/go-restful/v3"
)

func WrapF(f http.HandlerFunc) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		f(resp.ResponseWriter, req.Request)
	}
}

func WrapH(f http.Handler) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		f.ServeHTTP(resp.ResponseWriter, req.Request)
	}
}

func Router(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/debug/pprof")
	ws.Route(ws.GET("/").To(WrapF(pprof.Index)))
	ws.Route(ws.GET("/cmdline").To(WrapF(pprof.Cmdline)))
	ws.Route(ws.GET("/profile").To(WrapF(pprof.Profile)))
	ws.Route(ws.POST("/symbol").To(WrapF(pprof.Symbol)))
	ws.Route(ws.GET("/symbol").To(WrapF(pprof.Symbol)))
	ws.Route(ws.GET("/trace").To(WrapF(pprof.Trace)))
	ws.Route(ws.GET("/allocs").To(WrapH(pprof.Handler("allocs"))))
	ws.Route(ws.GET("/block").To(WrapH(pprof.Handler("block"))))
	ws.Route(ws.GET("/goroutine").To(WrapH(pprof.Handler("goroutine"))))
	ws.Route(ws.GET("/heap").To(WrapH(pprof.Handler("heap"))))
	ws.Route(ws.GET("/mutex").To(WrapH(pprof.Handler("mutex"))))
	ws.Route(ws.GET("/threadcreate").To(WrapH(pprof.Handler("threadcreate"))))
	container.Add(ws)
}
