package snbot

import (
	"fmt"

	"github.com/wenkaler/xfreehack/collector"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const info = `Доброго времени суток, вас приветствует xFree Bot!
Предназначенный собирать купоны и постить их в этот чат каждый день в 20:00 по МСК.
Купоны будут поступать по мере их нахождения. 
Если вы хотите получить прямо сейчас те купоны которые имеются у бота можете отправить команду /print5.`

type Storage interface {
	GetNotUseCoupon(cid int64) ([]collector.Record, error)
	MarkAsRead(cid int64, rr []collector.Record) error
	NewMessage(message *tgbotapi.Message) error
	NewChat(chat *tgbotapi.Chat) error
}

type Config struct {
	Logger     log.Logger
	Storage    Storage
	Token      string
	UpdateTime int
}

type SNBot struct {
	cfg *Config
	bot *tgbotapi.BotAPI
	upd tgbotapi.UpdatesChannel
}

func New(cfg *Config) (*SNBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, err
	}
	level.Info(cfg.Logger).Log("msg", "Authorized on account", "bot-name", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = cfg.UpdateTime
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		return nil, err
	}
	return &SNBot{
		cfg: cfg,
		bot: bot,
		upd: updates,
	}, nil
}

func (s *SNBot) read(message *tgbotapi.Message) error {
	err := s.cfg.Storage.NewMessage(message)
	if err != nil {
		return fmt.Errorf("failed create new message: %v", err)
	}
	var msg string
	switch message.Command() {
	case "start":
		err := s.cfg.Storage.NewChat(message.Chat)
		if err != nil {
			return fmt.Errorf("failed create new chat: %v", err)
		}
		msg = info
		s.Send(message.Chat.ID, msg)
	case "print5":
		records, err := s.cfg.Storage.GetNotUseCoupon(message.Chat.ID)
		if err != nil {
			return fmt.Errorf("failed get coupons: %v", err)
		}
		for i, rec := range records {
			msg = fmt.Sprintf("%v%v:\t%s \nКод--->: %s\nОписание: %s\n\n", msg, i+1, rec.Link, rec.Code, rec.Description)
		}
		if len(msg) == 0 {
			msg = `Вы получили все доступные купоны на данный момент.`
		}
		err = s.Send(message.Chat.ID, msg)
		if err != nil {
			return err
		}

		err = s.cfg.Storage.MarkAsRead(message.Chat.ID, records)
		if err != nil {
			return fmt.Errorf("failed marked as read: %v", err)
		}
	default:
		msg = info
		s.Send(message.Chat.ID, msg)
	}
	return nil
}

func (s *SNBot) Run() {
	for u := range s.upd {
		if u.Message == nil {
			continue
		}
		err := s.read(u.Message)
		if err != nil {
			level.Error(s.cfg.Logger).Log("msg", "failed read message", "err", err)
			s.Send(u.Message.Chat.ID, "service temporary unavailable")
		}

	}
}

func (s *SNBot) Send(chatID int64, msg string) error {
	var numericKeyboard = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/print5"),
		),
	)
	m := tgbotapi.NewMessage(chatID, msg)
	m.ReplyMarkup = numericKeyboard
	message, err := s.bot.Send(m)
	if err != nil {
		return fmt.Errorf("failed send message: %v", err)
	}
	err = s.cfg.Storage.NewMessage(&message)
	if err != nil {
		return fmt.Errorf("failed create message: %v", err)
	}
	return nil
}
