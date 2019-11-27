package main

import "github.com/reconquest/pkg/web"
import "net/http"

type WebHandler struct {
	web    *web.Web
	runner *Runner
}

func NewWebHandler(runner *Runner) *WebHandler {
	handler := &WebHandler{
		runner: runner,
	}
	web := web.New()

	web.Get("/", web.ServeFunc(handler.HandleDummy))

	handler.web = web

	return handler
}

func (handler *WebHandler) HandleDummy(context *web.Context) web.Status {
	context.Write([]byte("GET /"))

	return context.OK()
}

func (handler *WebHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	handler.web.ServeHTTP(response, request)
}
