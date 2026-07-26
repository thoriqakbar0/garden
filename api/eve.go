// Package handler exposes the canonical runtime through Vercel's native Go function runtime.
package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

var (
	once    sync.Once
	runtime http.Handler
)

// Handler serves all /eve/v1 routes after the vercel.json rewrite.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		root, err := os.Getwd()
		if err != nil {
			runtime = errorHandler(err)
			return
		}
		app, err := discover.ApplicationAt(root)
		if err != nil {
			runtime = errorHandler(err)
			return
		}
		store, err := workflow.Open(filepath.Join(os.TempDir(), "garden"), workflow.EchoResponder)
		if err != nil {
			runtime = errorHandler(err)
			return
		}
		runtime = server.Handler(app, store)
	})
	request := r.Clone(r.Context())
	if rewrittenPath := r.URL.Query().Get("path"); rewrittenPath != "" {
		request.URL.Path = "/eve/v1/" + rewrittenPath
	}
	runtime.ServeHTTP(w, request)
}

func errorHandler(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}
