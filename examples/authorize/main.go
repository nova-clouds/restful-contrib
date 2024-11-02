package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/iot-labs-team/restful-contrib/authorize"
)

type Account struct {
	Type     string `json:"type,omitempty"`
	Username string `json:"username,omitempty"`
}

func main() {
	auth, err := authorize.New[*Account](authorize.Config{
		Timeout:        time.Hour * 24,
		RefreshTimeout: time.Hour * (24 + 1),
		Lookup:         "header:Authorization:Bearer",
		Algorithm:      "HS256",
		Key:            "testSecretKey",
		PrivKey:        "",
		PubKey:         "",
		Issuer:         "gin-contrib",
	})
	if err != nil {
		panic(err)
	}

	ws := new(restful.WebService)
	ws.Filter(auth.Middleware(
		authorize.WithSkip(func(req *restful.Request, resp *restful.Response) bool {
			return req.Request.URL.Path == "/login"
		}),
		authorize.WithUnauthorizedFallback(func(req *restful.Request, resp *restful.Response, err error) {
			resp.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(resp, err.Error())
		}),
	))
	// curl -v http://127.0.0.1:8080/login -X 'POST' \
	//   --header 'Accept: */*' \
	//   --header 'Content-Type: application/json' \
	//   --header 'Accept-Encoding: gzip, deflate, br' \
	//   --header 'Connection: keep-alive' \
	//   --data-raw '{}' --insecure
	ws.Route(ws.POST("/login").To(func(req *restful.Request, resp *restful.Response) {
		tk, expiresAt, err := auth.GenerateToken(&authorize.Claims[*Account]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:      "1123",
				Subject: "1123",
			},
			Meta: &Account{
				Type:     "test",
				Username: "test",
			},
		})
		if err != nil {
			panic(err)
		}
		resp.WriteAsJson(map[string]any{
			"code":       http.StatusOK,
			"token":      tk,
			"expires_at": expiresAt.Unix(),
			"message":    "ok",
		})
	}))

	// curl -v http://127.0.0.1:8080/test-auth \
	//   --header 'Accept: */*' \
	//   --header 'Accept-Encoding: gzip, deflate, br' \
	//   --header 'Connection: keep-alive' \
	//   --header 'Authorization: Bearer {{realtoken}}' \
	//   --insecure
	ws.Route(ws.GET("/test-auth").To(func(req *restful.Request, resp *restful.Response) {
		resp.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(resp, "success auth")
	}))
	restful.Add(ws)

	log.Println("start server: 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
