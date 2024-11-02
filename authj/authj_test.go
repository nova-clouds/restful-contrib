package authj

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/emicklei/go-restful/v3"
)

func testAuthjRequest(t *testing.T, router http.Handler, user, path, method string, code int) {
	r, _ := http.NewRequestWithContext(context.TODO(), method, path, http.NoBody)
	r.SetBasicAuth(user, "123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != code {
		t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, w.Code, code)
	}
}

func TestBasic(t *testing.T) {
	e, _ := casbin.NewEnforcer("authj_model.conf", "authj_policy.csv")

	router := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		ContextWithSubject(req, resp, "alice")
		chain.ProcessFilter(req, resp)
	})
	ws.Filter(Authorizer(e, WithSubject(Subject)))

	okfunc := func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
	}
	ws.Route(ws.GET("/{anypath:*}").To(okfunc))
	ws.Route(ws.POST("/{anypath:*}").To(okfunc))
	router.Add(ws)

	testAuthjRequest(t, router, "alice", "/dataset1/resource1", "GET", 200)
	testAuthjRequest(t, router, "alice", "/dataset1/resource1", "POST", 200)
	testAuthjRequest(t, router, "alice", "/dataset1/resource2", "GET", 200)
	testAuthjRequest(t, router, "alice", "/dataset1/resource2", "POST", 403)
}

func TestPathWildcard(t *testing.T) {
	e, _ := casbin.NewEnforcer("authj_model.conf", "authj_policy.csv")

	router := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		ContextWithSubject(req, resp, "bob")
		chain.ProcessFilter(req, resp)
	})
	ws.Filter(Authorizer(e, WithSubject(Subject)))

	okfunc := func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
	}
	ws.Route(ws.GET("/{anypath:*}").To(okfunc))
	ws.Route(ws.POST("/{anypath:*}").To(okfunc))
	ws.Route(ws.DELETE("/{anypath:*}").To(okfunc))
	router.Add(ws)

	testAuthjRequest(t, router, "bob", "/dataset2/resource1", "GET", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/resource1", "POST", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/resource1", "DELETE", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/resource2", "GET", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/resource2", "POST", 403)
	testAuthjRequest(t, router, "bob", "/dataset2/resource2", "DELETE", 403)

	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item1", "GET", 403)
	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item1", "POST", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item1", "DELETE", 403)
	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item2", "GET", 403)
	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item2", "POST", 200)
	testAuthjRequest(t, router, "bob", "/dataset2/folder1/item2", "DELETE", 403)
}

func TestRBAC(t *testing.T) {
	e, _ := casbin.NewEnforcer("authj_model.conf", "authj_policy.csv")

	router := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		ContextWithSubject(req, resp, "cathy")
		chain.ProcessFilter(req, resp)
	})
	ws.Filter(Authorizer(e,
		WithSubject(Subject),
		WithErrorFallback(func(req *restful.Request, resp *restful.Response, err error) {
			resp.WriteHeaderAndJson(
				http.StatusInternalServerError, map[string]any{
					"code": http.StatusInternalServerError,
					"msg":  "Permission validation errors occur!",
				},
				restful.MIME_JSON,
			)
		}),
		WithForbiddenFallback(func(req *restful.Request, resp *restful.Response) {
			resp.WriteHeaderAndJson(
				http.StatusForbidden, map[string]any{
					"code": http.StatusForbidden,
					"msg":  "Permission denied!",
				},
				restful.MIME_JSON,
			)
		}),
	))
	okfunc := func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
	}
	ws.Route(ws.GET("/{anypath:*}").To(okfunc))
	ws.Route(ws.POST("/{anypath:*}").To(okfunc))
	ws.Route(ws.DELETE("/{anypath:*}").To(okfunc))
	router.Add(ws)

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "GET", 200)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "POST", 200)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "DELETE", 200)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "DELETE", 403)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	_, _ = e.DeleteRolesForUser("cathy")

	testAuthjRequest(t, router, "cathy", "/dataset1/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "DELETE", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "DELETE", 403)
}

func TestSkipAuthentication(t *testing.T) {
	e, _ := casbin.NewEnforcer("authj_model.conf", "authj_policy.csv")

	router := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		ContextWithSubject(req, resp, "cathy")
		chain.ProcessFilter(req, resp)
	})
	ws.Filter(Authorizer(e,
		WithSubject(Subject),
		WithErrorFallback(func(req *restful.Request, resp *restful.Response, err error) {
			resp.WriteHeaderAndJson(
				http.StatusInternalServerError, map[string]any{
					"code": http.StatusInternalServerError,
					"msg":  "Permission validation errors occur!",
				},
				restful.MIME_JSON,
			)
		}),
		WithForbiddenFallback(func(req *restful.Request, resp *restful.Response) {
			resp.WriteHeaderAndJson(
				http.StatusForbidden, map[string]any{
					"code": http.StatusForbidden,
					"msg":  "Permission denied!",
				},
				restful.MIME_JSON,
			)
		}),
		WithSkipAuthentication(func(req *restful.Request, resp *restful.Response) bool {
			if req.Request.Method == http.MethodGet && req.Request.URL.Path == "/skip/authentication" {
				return true
			}
			return false
		}),
	))
	okfunc := func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(200)
	}
	ws.Route(ws.GET("/{anypath:*}").To(okfunc))
	ws.Route(ws.POST("/{anypath:*}").To(okfunc))
	ws.Route(ws.DELETE("/{anypath:*}").To(okfunc))
	router.Add(ws)

	// skip authentication
	testAuthjRequest(t, router, "cathy", "/skip/authentication", "GET", 200)
	testAuthjRequest(t, router, "cathy", "/skip/authentication", "POST", 403)

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "GET", 200)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "POST", 200)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "DELETE", 200)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "DELETE", 403)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	_, _ = e.DeleteRolesForUser("cathy")

	testAuthjRequest(t, router, "cathy", "/dataset1/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset1/item", "DELETE", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "GET", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "POST", 403)
	testAuthjRequest(t, router, "cathy", "/dataset2/item", "DELETE", 403)
}
