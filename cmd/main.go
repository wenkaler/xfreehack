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
	PathDB      string `envconfig:"path_db" default:"/db/xfree.db"`
	TimeToSend  string `envconfig:"time_to_send" default:"18:00"`
	Telegram    struct {
		Token      string `envconfig:"telegram_token" required:"true"`
		UpdateTime int    `envconfig:"telegram_update_bot" default:"60"`
	}
	AccessToken string `envconfig:"access_token" required:"true"`
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
		Logger:      logger,
		Storage:     s,
		Token:       cfg.Telegram.Token,
		UpdateTime:  cfg.Telegram.UpdateTime,
		AccessToken: cfg.AccessToken,
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
		level.Error(logger).Log("msg", "failed create collector", "err", err)
		os.Exit(1)
	}
	go sn.Run()
	c.Collect(collector.ConditionQuery{
		URI: "https://lovikod.ru/knigi/promokody-litres",
	})
	gocron.Every(1).Days().At(cfg.TimeToSend).Do(task, sn, s, c, logger)
	cronCh := gocron.Start()

	cl := make(chan os.Signal, 1)
	signal.Notify(cl, syscall.SIGTERM, syscall.SIGINT)
	sig := <-cl
	cronCh <- true
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
		level.Error(logger).Log("msg", "failed get chats", "err", err)
	}
	for _, id := range chats {
		err := bot.SendCoupons(id, "", snbot.Daily)
		if err != nil {
			level.Error(logger).Log("msg", "failed send coupons", "chatID", id, "err", err)
			continue
		}
	}
	level.Info(logger).Log("msg", "send all chats new coupons")
}
