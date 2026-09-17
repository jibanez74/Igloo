//go:build pprofdebug

package main

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// registerPprof mounts runtime profiling endpoints under /api/debug/pprof.
// It only exists in builds with the pprofdebug tag, so release and test
// builds never expose these routes.
func (app *Application) registerPprof(r chi.Router) {
	r.Route("/debug/pprof", func(r chi.Router) {
		r.Use(app.RequireAdmin)
		r.Get("/", pprof.Index)
		r.Get("/cmdline", pprof.Cmdline)
		r.Get("/profile", pprof.Profile)
		r.Get("/symbol", pprof.Symbol)
		r.Post("/symbol", pprof.Symbol)
		r.Get("/trace", pprof.Trace)
		r.Get("/{profile}", func(w http.ResponseWriter, req *http.Request) {
			pprof.Handler(chi.URLParam(req, "profile")).ServeHTTP(w, req)
		})
	})
}
