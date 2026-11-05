package main

import (
	"net/http"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.notFound)

	mux.HandleFunc("GET /status", app.status)

	mux.Handle("GET /restricted-basic-auth", app.requireBasicAuthentication(http.HandlerFunc(app.restricted)))

	return app.logRequest(app.recoverPanic(mux))
}
