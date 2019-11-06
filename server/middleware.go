package server

import (
	"github.com/go-kit/kit/log"
)

// loggingMiddleware wraps Service and logs request information to the provided logger.
type loggingMiddleware struct {
	log.Logger
	next service
}
