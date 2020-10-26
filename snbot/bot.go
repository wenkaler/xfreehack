package snbot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wenkaler/xfreehack/collector"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const info = `Доброго времени суток, вас приветствует xFree Bot!
Предназначенный собирать купоны и постить их в этот чат каждый день в 18:00 по МСК.
Купоны будут поступать по мере их нахождения. 
Если вы хотите получить прямо сейчас те купоны которые имеются у бота можете отправить команду /print 5 (кол-во купонов по умолчанию 5).
https://t.me/XFRebot - группа в которой можно задать вопросы по боту.`

const errBlockedByUser = "Forbidden: bot was blocked by the user"

type Storage interface {
	GetNotUseCoupon(cid int64) ([]collector.Record, error)
	GetNotUseCouponCount(cid, count int64) ([]collector.Record, error)
	GetCountUser() (int, error)
	CountNotUseCoupon(cid int64) (uint64, error)
	MarkAsRead(cid int64, rr []collector.Record) error
	NewChat(chat *tgbotapi.Chat) error
	UpdChatActivity(cid int64, act bool) error
}

type Config struct {
	Logger      log.Logger
	Storage     Storage
	Token       string
	UpdateTime  int
	AccessToken string
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

type reqType int

const (
	Command reqType = 0
	Daily   reqType = 1
)

func (s *SNBot) SendCoupons(chatID int64, cmdArgs string, t reqType) error {
	var count int64 = 5
	var msg string
	if strings.TrimSpace(cmdArgs) != "" {
		ss := strings.Split(cmdArgs, " ")
		c, err := strconv.ParseInt(ss[0], 10, 64)
		if err == nil {
			count = c
		}
	}
	records, err := s.cfg.Storage.GetNotUseCouponCount(chatID, count)
	if err != nil {
		return fmt.Errorf("failed get coupons: %v", err)
	}
	for i, rec := range records {
		msg = fmt.Sprintf("%v%v:\t%s \nКод--->: %s\nВремя истечения: %v\nОписание: %s\n\n", msg, i+1, rec.Link, rec.Code, time.Unix(rec.Date, 0).Format("02.01.2006"), rec.Description)
	}
	if len(msg) == 0 && t == Command {
		msg = `Вы получили все доступные купоны на данный момент.`
	}
	err = s.Send(chatID, msg)
	if err != nil {
		return err
	}

	err = s.cfg.Storage.MarkAsRead(chatID, records)
	if err != nil {
		return fmt.Errorf("failed marked as read: %v", err)
	}
	cc, err := s.cfg.Storage.CountNotUseCoupon(chatID)
	if err != nil {
		level.Error(s.cfg.Logger).Log("msg", "failed get count coupons", "chatID", chatID, "err", err)
	} else if cc != 0 {
		msg := fmt.Sprintf("Купоны оставшиеся в базе: %v", cc)
		err = s.Send(chatID, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SNBot) read(message *tgbotapi.Message) error {
	var msg string
	switch message.Command() {
	case "start":
		err := s.cfg.Storage.NewChat(message.Chat)
		if err != nil {
			return fmt.Errorf("failed create new chat: %v", err)
		}
		msg = info
		s.Send(message.Chat.ID, msg)
	case "print":
		err := s.SendCoupons(message.Chat.ID, message.CommandArguments(), Command)
		if err != nil {
			return err
		}
	case "stat":
		err := s.SendStat(message.Chat.ID, message.CommandArguments())
		if err != nil {
			return err
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
	level.Error(s.cfg.Logger).Log("msg", "try send", "chatID", chatID)
	var numericKeyboard = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/print"),
		),
	)
	m := tgbotapi.NewMessage(chatID, msg)
	m.ReplyMarkup = numericKeyboard
	_, err := s.bot.Send(m)
	if err != nil {
		if err.Error() == errBlockedByUser {
			s.cfg.Storage.UpdChatActivity(chatID, false)
		}
		return err
	}
	return nil
}

func (s *SNBot) SendStat(chatID int64, args string) error {
	if strings.TrimSpace(args) != "" {
		ss := strings.Split(args, " ")
		token := ss[0]
		if s.cfg.AccessToken != token {
			return errors.New("failed token")
		}
		count, err := s.cfg.Storage.GetCountUser()
		if err != nil {
			return err
		}
		s.Send(chatID, fmt.Sprintf("Активных пользователей в базе: %d", count))
	}
	return nil
}
