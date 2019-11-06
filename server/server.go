package server

import (
	"github.com/go-kit/kit/log"
	"net/http"
)

type Config struct{
	Logger log.Logger
	Storage Storage
	Port string
}

type Storage interface{

}

type Server struct{
	src *basicService
	srv *http.Server
	cfg *Config
}

func New(cfg *Config) (*Server, error){
	svc := &basicService{
		logger:  cfg.Logger,
		storage: cfg.Storage,
	}

	handler := newHandler(&handlerConfig{})
	return nil, nil
}