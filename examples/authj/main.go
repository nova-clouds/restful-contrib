package main

import (
	"io"
	"log"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/emicklei/go-restful/v3"

	"github.com/iot-sonata/restful-contrib/authj"
)

func main() {
	// load the casbin model and policy from files, database is also supported.
	e, err := casbin.NewEnforcer("../../authj/authj_model.conf", "../../authj/authj_policy.csv")
	if err != nil {
		panic(err)
	}

	// define your router, and use the Casbin authj middleware.
	// the access that is denied by authj will return HTTP 403 error.
	ws := new(restful.WebService)
	ws.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		authj.ContextWithSubject(req, resp, "alice")
		chain.ProcessFilter(req, resp)
	})
	ws.Filter(authj.Authorizer(e))
	// curl -v http://127.0.0.1:8080/dataset1/resource1 \
	//   --header 'Accept: */*' \
	//   --header 'Accept-Encoding: gzip, deflate, br' \
	//   --header 'Connection: keep-alive' \
	//   --insecure
	ws.Route(ws.GET("/dataset1/resource1").To(func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
		_, _ = io.WriteString(resp, "alice own this resource")
	}))
	// curl -v http://127.0.0.1:8080/dataset2/resource1 \
	//   --header 'Accept: */*' \
	//   --header 'Accept-Encoding: gzip, deflate, br' \
	//   --header 'Connection: keep-alive' \
	//   --insecure
	ws.Route(ws.GET("/dataset2/resource1").To(func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
		_, _ = io.WriteString(resp, "alice do not own this resource")
	}))
	restful.Add(ws)

	log.Println("start server: 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
