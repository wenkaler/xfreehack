package server

import (
	"net/http"

	"github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type handlerConfig struct {
	svc    service
	logger log.Logger
}

// newHandler creates a new HTTP handler serving service endpoints.
func newHandler(cfg *handlerConfig) http.Handler {

	svc := &loggingMiddleware{Logger: cfg.logger, next: cfg.svc}

	// log only request decoding/rate-limit errors.
	opts := []kithttp.ServerOption{
		kithttp.ServerErrorLogger(cfg.logger),
	}

	router := mux.NewRouter()

	getUsersByRVEndpoint := makeGetUsersByRVEndpoint(svc)
	router.Path("/api/v1/users/{rv}").Methods(http.MethodGet).Handler(kithttp.NewServer(
		getUsersByRVEndpoint,
		decodeGetUsersByRVRequest,
		encodeGetUsersByRVResponse,
		opts...,
	))

	return router
}
