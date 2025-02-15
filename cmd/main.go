package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jasonlvhit/gocron"
	"github.com/spf13/viper"

	"github.com/wenkaler/xfreehack/internal/bot"
	"github.com/wenkaler/xfreehack/internal/collector"
	"github.com/wenkaler/xfreehack/internal/config"
	"github.com/wenkaler/xfreehack/internal/storage"
)

var serviceVersion = "dev"

func main() {
	// Create a logger instance with JSON formatting
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Unmarshal config into a struct
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Error("viper.Unmarshal:", slog.Any("error", err))
		os.Exit(1)
	}

	store, err := storage.New(cfg.PathDB)
	if err != nil {
		logger.Error("storage.New", slog.Any("error", err))
		os.Exit(1)
	}

	sn, err := bot.New(
		cfg.TBot.AccessToken,
		cfg.TBot.AdminChatID,
		store,
		cfg.TBot.UpdateConfig,
	)
	if err != nil {
		logger.Error("bot.New", slog.Any("error", err))
		os.Exit(1)
	}

	c, err := collector.New(&collector.Config{
		Storage: store,
	})
	if err != nil {
		logger.Error("collector.New", slog.Any("error", err))
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
	logger.Info("received signal, exiting", slog.Any("sign", sig))
	store.Close()

	logger.Info("msg", "goodbye")
}

func task(bot *bot.TBot, s *storage.Storage, c *collector.Collector, logger *slog.Logger) {
	c.Collect(collector.ConditionQuery{
		URI: "https://lovikod.ru/knigi/promokody-litres",
	})
	chats, err := s.GetChat()
	if err != nil {
		logger.Error("failed get chats", slog.Any("error", err))
	}
	for _, id := range chats {
		err := bot.SendCoupons(id, "", bot.Daily)
		if err != nil {
			logger.Error("failed send coupons", slog.Any("error", err))
			continue
		}
	}
	logger.Info("send all chats new coupons")
}
