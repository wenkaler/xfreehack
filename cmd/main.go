package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wenkaler/xfreehack/snbot"
	"gopkg.in/yaml.v3"

	"github.com/jasonlvhit/gocron"

	"github.com/wenkaler/xfreehack/storage"

	"github.com/kelseyhightower/envconfig"
	"github.com/wenkaler/xfreehack/collector"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type configure struct {
	ServiceName string `yaml:"service_name" envconfig:"service_name" default:"xFreeService"`
	DB          struct {
		Host     string `yaml:"host" envconfig:"db_host" default:"localhost"`
		Port     string `yaml:"port" envconfig:"db_port" default:"5432"`
		User     string `yaml:"user" envconfig:"db_user" default:"postgres"`
		Password string `yaml:"password" envconfig:"db_password" default:"secret"`
		Name     string `yaml:"name" envconfig:"db_name" default:"postgres"`
		SSLMode  string `yaml:"ssl_mode" envconfig:"db_ssl_mode" default:"disable"`
	} `yaml:"db"`
	TimeToSend string `yaml:"time_to_send" envconfig:"time_to_send" default:"18:00"`
	Telegram   struct {
		Token      string `yaml:"token" envconfig:"telegram_token" required:"true"`
		UpdateTime int    `yaml:"update_time" envconfig:"telegram_update_bot" default:"60"`
	} `yaml:"telegram"`
	AccessToken string `yaml:"access_token" envconfig:"access_token" required:"true"`
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

	// 1. Try to load from config.yaml
	if _, err := os.Stat("config.yaml"); err == nil {
		data, err := ioutil.ReadFile("config.yaml")
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				level.Error(logger).Log("msg", "failed to parse config.yaml", "err", err)
			} else {
				level.Info(logger).Log("msg", "loaded config from config.yaml")
			}
		}
	}

	// 2. Override with Env Vars
	err := envconfig.Process("", &cfg)
	if err != nil {
		level.Error(logger).Log("msg", "failed to load configuration from env", "err", err)
		os.Exit(1)
	}

	// Construct DSN
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name, cfg.DB.SSLMode)

	s, err := storage.New(dsn, logger)
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

	// Collector New now returns only *Collector
	c := collector.New(&collector.Config{
		Logger:  logger,
		Storage: s,
	})

	go sn.Run()

	// Run initial collection
	if err := c.CollectAll(); err != nil {
		level.Error(logger).Log("msg", "failed initial collection", "err", err)
	}

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
	if err := c.CollectAll(); err != nil {
		level.Error(logger).Log("msg", "failed scheduled collection", "err", err)
	}

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
