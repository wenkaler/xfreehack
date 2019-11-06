package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/wenkaler/xfreehack/collector"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type configure struct {
	ServiceName string `envconfig:"service_name"`
	Port        string `envconfig:"port"`
}

var serviceVersion = "dev"

func main() {
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *printVersion {
		fmt.Println(serviceVersion)
		os.Exit(0)
	}

	logger := kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stderr))
	logger = kitlog.With(logger, "caller", kitlog.DefaultCaller)
	log.SetOutput(kitlog.NewStdlibAdapter(logger))
	logger = kitlog.With(logger, "ts", kitlog.DefaultTimestampUTC)

	var cfg configure
	err := envconfig.Process("", &cfg)
	if err != nil {
		level.Error(logger).Log("msg", "failed to load configuration", "err", err)
		os.Exit(1)
	}

	c, err := collector.New(&collector.Config{
		Logger:      logger,
		Storage:     nil,
		URL:         "https://halyavshiki.com/",
		NameMarkets: []string{"ЛитРес"},
	})
	if err != nil {
		level.Error(logger).Log("msg", "failed create collectore", "err", err)
		os.Exit(1)
	}
	c.Collect()
}
