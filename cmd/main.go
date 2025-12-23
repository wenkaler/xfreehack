package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Schedule:
	// 1. Collection every 15 minutes
	gocron.Every(15).Minutes().Do(func() {
		collectTask(c, logger)
	})

	// 2. Notify users with 'immediate' schedule every 15 minutes (after collection)
	gocron.Every(15).Minutes().Do(func() {
		notifyTask(sn, s, logger, "immediate")
	})

	// 3. Notify users with '18:00' schedule daily at configured time
	gocron.Every(1).Days().At(cfg.TimeToSend).Do(func() {
		notifyTask(sn, s, logger, "18:00")
	})

	// 4. Check for system notifications every 1 minute
	gocron.Every(1).Minutes().Do(func() {
		checkNotifications(sn, s, logger)
	})

	cronCh := gocron.Start()

	cl := make(chan os.Signal, 1)
	signal.Notify(cl, syscall.SIGTERM, syscall.SIGINT)
	sig := <-cl
	cronCh <- true
	level.Info(logger).Log("msg", "received signal, exiting", "signal", sig)
	s.Close()

	level.Info(logger).Log("msg", "goodbye")
}

func collectTask(c *collector.Collector, logger kitlog.Logger) {
	level.Info(logger).Log("msg", "starting scheduled collection")
	if err := c.CollectAll(); err != nil {
		level.Error(logger).Log("msg", "failed scheduled collection", "err", err)
	}
}

func notifyTask(bot *snbot.SNBot, s *storage.Storage, logger kitlog.Logger, schedule string) {
	level.Info(logger).Log("msg", "starting notification task", "schedule", schedule)

	chats, err := s.GetChatsBySchedule(schedule)
	if err != nil {
		level.Error(logger).Log("msg", "failed get chats", "err", err)
		return
	}

	for _, id := range chats {
		err := bot.SendCoupons(id, "", snbot.Daily)
		if err != nil {
			level.Error(logger).Log("msg", "failed send coupons", "chatID", id, "err", err)
			continue
		}
	}
	level.Info(logger).Log("msg", "finished notification task", "schedule", schedule, "chats_count", len(chats))
}

func checkNotifications(bot *snbot.SNBot, s *storage.Storage, logger kitlog.Logger) {
	nots, err := s.GetPendingNotifications()
	if err != nil {
		level.Error(logger).Log("msg", "failed to check notifications", "err", err)
		return
	}

	if len(nots) == 0 {
		return
	}

	level.Info(logger).Log("msg", "found pending notifications", "count", len(nots))

	for _, n := range nots {
		// Log start
		level.Info(logger).Log("msg", "sending notification", "id", n.ID, "target", n.TargetSegment)

		var chats []int64
		// For now support 'all'
		if n.TargetSegment == "all" || n.TargetSegment == "" {
			chats, err = s.GetChat()
		} else {
			// specific segment logic (e.g. 'migrated' could be same as all for now or tracked elsewhere)
			// fallback to all for safety or skip
			level.Warn(logger).Log("msg", "unknown target segment, defaulting to all", "segment", n.TargetSegment)
			chats, err = s.GetChat()
		}

		if err != nil {
			level.Error(logger).Log("msg", "failed to get chats for notification", "err", err)
			continue
		}

		successCount := 0
		for _, chatID := range chats {
			if err := bot.Send(chatID, n.Message); err != nil {
				level.Error(logger).Log("msg", "failed to send notification", "chatID", chatID, "err", err)
			} else {
				successCount++
			}
			// Simple rate limiting
			time.Sleep(50 * time.Millisecond)
		}

		if err := s.MarkNotificationSent(n.ID); err != nil {
			level.Error(logger).Log("msg", "failed to mark notification as sent", "id", n.ID, "err", err)
		}
		level.Info(logger).Log("msg", "finished sending notification", "id", n.ID, "sent_count", successCount)
	}
}
