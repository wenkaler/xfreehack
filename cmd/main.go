package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wenkaler/xfreehack/snbot"

	"github.com/jasonlvhit/gocron"

	"github.com/wenkaler/xfreehack/storage"

	"github.com/kelseyhightower/envconfig"
	"github.com/wenkaler/xfreehack/collector"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type configure struct {
	ServiceName string `envconfig:"service_name" default:"xFreeService"`
	PathDB      string `envconfig:"path_db" default:"/xfreehack-db/xfree.db"`
	Telegram    struct {
		Token      string `envconfig:"telegram_token" required:"true"`
		UpdateTime int    `envconfig:"telegram_update_bot" default:"60"`
	}
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
	s, err := storage.New(cfg.PathDB, logger)
	if err != nil {
		level.Error(logger).Log("msg", "failed create storage", "err", err)
		os.Exit(1)
	}

	sn, err := snbot.New(&snbot.Config{
		Logger:     logger,
		Storage:    s,
		Token:      cfg.Telegram.Token,
		UpdateTime: cfg.Telegram.UpdateTime,
	})
	if err != nil {
		level.Error(logger).Log("msg", "failed create bot", "err", err)
		os.Exit(1)
	}

	c, err := collector.New(&collector.Config{
		Logger:  logger,
		Storage: s,
	})
	if err != nil {
		level.Error(logger).Log("msg", "failed create collectore", "err", err)
		os.Exit(1)
	}
	sn.Run()
	c.Collect(collector.ConditionQuery{
		URI: "https://lovikod.ru/knigi/promokody-litres",
	})
	gocron.Every(1).Days().At("20:00").Do(task, sn, s, c)
	gocron.Start()

	cl := make(chan os.Signal, 1)
	signal.Notify(cl, syscall.SIGTERM, syscall.SIGINT)
	sig := <-cl

	level.Info(logger).Log("msg", "received signal, exiting", "signal", sig)
	s.Close()

	level.Info(logger).Log("msg", "goodbye")
}

func task(bot *snbot.SNBot, s *storage.Storage, c *collector.Collector, logger kitlog.Logger) {
	c.Collect(collector.ConditionQuery{
		URI: "https://lovikod.ru/knigi/promokody-litres",
	})
	chats, err := s.GetChat()
	if err != nil {
		log.Println("failed get chats")
	}
	for _, id := range chats {
		records, err := s.GetNotUseCoupon(id)
		if err != nil {
			level.Error(logger).Log("msg", "failed get coupons", "err", err)
			return
		}
		var msg string
		for _, rec := range records {
			msg += fmt.Sprintf("%s - %s \n", rec.Link, rec.Code)
		}
		if msg != "" {
			err := bot.Send(id, msg)
			if err != nil {
				level.Error(logger).Log("msg", "failed send message", "err", err)
				continue
			}
			err = s.MarkAsRead(id, records)
			if err != nil {
				level.Error(logger).Log("msg", "failed marked as read", "err", err)
				continue
			}
		}
	}
}
