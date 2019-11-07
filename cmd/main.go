package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	"github.com/wenkaler/xfreehack/storage"

	"github.com/kelseyhightower/envconfig"
	"github.com/wenkaler/xfreehack/collector"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/jasonlvhit/gocron"
)

type configure struct {
	ServiceName  string   `envconfig:"service_name" default:"xFreeService"`
	PathDB       string   `envconfig:"path_db" default:"xfree.db"`
	MarketPlaces []string `envconfig:"market_places" default:"ЛитРес"`
	URL          string   `envconfig:"url" default:"https://halyavshiki.com/"`

	Telegram struct {
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

	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		level.Error(logger).Log("msg", "failed create bot", "err", err)
		os.Exit(1)
	}
	level.Info(logger).Log("msg", "Authorized on account", "bot-name", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = cfg.Telegram.UpdateTime
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		level.Error(logger).Log("msg", "failed ged update bot chanel", "err", err)
		os.Exit(1)
	}
	go func(upd tgbotapi.UpdatesChannel, s *storage.Storage) {
		for u := range upd {
			if u.Message == nil {
				continue
			}
			if u.Message.Command() == "start" {
				fmt.Println(u.Message.From.String())
				err := s.NewChat(u.Message.Chat.ID)
				if err != nil {
					log.Printf("failed create new chat: %v", err)
					continue
				}
				m := tgbotapi.NewMessage(u.Message.Chat.ID, `Бот создан для поиска купонов от ЛитРес.
Введите команду для приемлимой конфигурации бота. 
/only_new - для получения только новыйх купонов
/all - для получения всех купонов которые есть в базе
После конфигураци вы будете автоматически получать купон(ы) в 20:00 по МСК`)
				_, err = bot.Send(m)
				if err != nil {
					log.Println("failed send message: %v", err)
					continue
				}
			}
			if u.Message.Command() == "only_new" {
				records, err := s.GetNotUseCoupon(u.Message.Chat.ID)
				if err != nil {
					log.Printf("failed get coupons: %v", err)
				}
				m := tgbotapi.NewMessage(u.Message.Chat.ID, `Настройка завершена`)
				bot.Send(m)
				err = s.MarkAsRead(u.Message.Chat.ID, records)
				if err != nil {
					log.Printf("failed marked as read: %v", err)
				}
			}
			if u.Message.Command() == "all" {
				records, err := s.GetNotUseCoupon(u.Message.Chat.ID)
				if err != nil {
					log.Printf("failed get coupons: %v", err)
				}
				var msg = `Настройка завершена`
				for _, rec := range records {
					msg = fmt.Sprintf("%v\n%s - %s - %s", msg, rec.Market, rec.Link, rec.Code)
				}
				m := tgbotapi.NewMessage(u.Message.Chat.ID, msg)
				_, err = bot.Send(m)
				if err != nil {
					log.Printf("failed send message: %v", err)
				}
				err = s.MarkAsRead(u.Message.Chat.ID, records)
				if err != nil {
					log.Printf("failed marked as read: %v", err)
				}

			}
		}
	}(updates, s)
	c, err := collector.New(&collector.Config{
		Logger:  logger,
		Storage: s,
		//Bot:         bot,
		URL:         cfg.URL,
		NameMarkets: cfg.MarketPlaces,
	})
	if err != nil {
		level.Error(logger).Log("msg", "failed create collectore", "err", err)
		os.Exit(1)
	}
	c.Collect()
	gocron.Every(1).Days().At("20:00").Do(task, bot, s, c)
	cl := make(chan os.Signal, 1)
	signal.Notify(cl, syscall.SIGTERM, syscall.SIGINT)
	sig := <-cl

	level.Info(logger).Log("msg", "received signal, exiting", "signal", sig)
	s.Close()

	level.Info(logger).Log("msg", "goodbye")
}

func task(bot *tgbotapi.BotAPI, s *storage.Storage, c *collector.Collector) {
	c.Collect()
	chats, err := s.GetChat()
	if err != nil {
		log.Println("failed get chats")
	}
	for _, id := range chats {
		records, err := s.GetNotUseCoupon(id)
		if err != nil {
			log.Printf("failed get coupons: %v", err)
		}
		var msg string
		for _, rec := range records {
			msg = fmt.Sprintf("%v\n%s - %s - %s", msg, rec.Market, rec.Link, rec.Code)
		}
		m := tgbotapi.NewMessage(id, msg)
		_, err = bot.Send(m)
		if err != nil {
			log.Printf("failed send message: %v", err)
		}
		err = s.MarkAsRead(id, records)
		if err != nil {
			log.Printf("failed marked as read: %v", err)
		}
	}
}
