//go:build !pprofdebug

package main

import "github.com/go-chi/chi/v5"

// registerPprof is a no-op without the pprofdebug build tag.
func (app *Application) registerPprof(r chi.Router) {}
