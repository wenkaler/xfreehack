package server

import "github.com/go-kit/kit/log"

type service interface{}

type basicService struct {
	logger  log.Logger
	storage Storage
}
