package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tba "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/wenkaler/xfreehack/internal/config"
	"github.com/wenkaler/xfreehack/internal/model"
)

type reqType int

const (
	Command reqType = 0
	Daily   reqType = 1
)

var numericKeyboard = tba.NewReplyKeyboard(
	tba.NewKeyboardButtonRow(
		tba.NewKeyboardButton("/print"),
	),
)

const info = `Доброго времени суток, вас приветствует xFree Bot!
Предназначенный собирать купоны и постить их в этот чат каждый день в 18:00 по МСК.
Купоны будут поступать по мере их нахождения. 
Если вы хотите получить прямо сейчас те купоны которые имеются у бота можете отправить команду /print 5 (кол-во купонов по умолчанию 5).
https://t.me/XFRebot - группа в которой можно задать вопросы по боту.`

const errBlockedByUser = "Forbidden: bot was blocked by the user"

type Getter interface {
	GetNotUseCoupon(cid int64) ([]model.Record, error)
	GetNotUseCouponCount(cid, count int64) ([]model.Record, error)
	GetCountUser() (int, error)
	CountNotUseCoupon(cid int64) (uint64, error)
	MarkAsRead(cid int64, rr []model.Record) error
	NewChat(chat *tba.Chat) error
	UpdChatActivity(cid int64, act bool) error
}

type TBot struct {
	adminChatID int64
	getter      Getter
	bot         *tba.BotAPI
	upd         tba.UpdatesChannel
}

func New(
	accessToken string,
	adminChatID int64,
	g Getter,
	upc config.UpdateConfig,
) (*TBot, error) {
	tb, err := tba.NewBotAPI(accessToken)
	if err != nil {
		return nil, fmt.Errorf("tba.NewBotAPI: %w", err)
	}
	updates, err := tb.GetUpdatesChan(tba.UpdateConfig{
		Offset:  upc.Offset,
		Limit:   upc.Limit,
		Timeout: upc.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("tba.GetUpdatesChan: %w", err)
	}

	return &TBot{
		adminChatID: adminChatID,
		getter:      nil,
		bot:         tb,
		upd:         updates,
	}, nil
}

func (tb *TBot) SendCoupons(chatID int64, cmdArgs string, t reqType) error {
	var count int64 = 5
	var msg string
	if strings.TrimSpace(cmdArgs) != "" {
		ss := strings.Split(cmdArgs, " ")
		c, err := strconv.ParseInt(ss[0], 10, 64)
		if err == nil {
			count = c
		}
	}
	records, err := tb.getter.GetNotUseCouponCount(chatID, count)
	if err != nil {
		return fmt.Errorf("failed get coupons: %v", err)
	}
	for i, rec := range records {
		msg = fmt.Sprintf("%v%v:\t%s \nКод--->: %s\nВремя истечения: %v\nОписание: %s\n\n", msg, i+1, rec.Link, rec.Code, time.Unix(rec.Date, 0).Format("02.01.2006"), rec.Description)
	}
	if len(msg) == 0 && t == Command {
		msg = `Вы получили все доступные купоны на данный момент.`
	}
	_, err = tb.send(chatID, msg)
	if err != nil {
		return err
	}

	err = tb.getter.MarkAsRead(chatID, records)
	if err != nil {
		return fmt.Errorf("failed marked as read: %v", err)
	}
	cc, err := tb.getter.CountNotUseCoupon(chatID)
	if err != nil {
		return fmt.Errorf("tb.getter.CountNotUseCoupon: %w", err)
	}

	if cc != 0 {
		msg := fmt.Sprintf("Купоны оставшиеся в базе: %v", cc)
		_, err = tb.send(chatID, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tb *TBot) read(message *tba.Message) error {
	var msg string
	switch message.Command() {
	case "start":
		err := tb.getter.NewChat(message.Chat)
		if err != nil {
			return fmt.Errorf("failed create new chat: %v", err)
		}
		msg = info
	case "print":
		err := tb.SendCoupons(message.Chat.ID, message.CommandArguments(), Command)
		if err != nil {
			return err
		}
		return nil
	default:
		msg = info
	}
	_, err := tb.send(message.Chat.ID, msg)
	if err != nil {
		return fmt.Errorf("s.send: %w", err)
	}

	return nil
}

func (tb *TBot) Run() {
	for u := range tb.upd {
		if u.Message == nil {
			continue
		}
		err := tb.read(u.Message)
		if err != nil {
			tb.send(u.Message.Chat.ID, "service temporary unavailable")
		}

	}
}

func (tb *TBot) sendStat(msg string) error {
	_, err := tb.send(tb.adminChatID, msg)
	if err != nil {
		return fmt.Errorf("tb.send: %w", err)
	}
	return nil
}

func (tb *TBot) send(chatID int64, msg string) (*tba.Message, error) {
	m := tba.NewMessage(chatID, msg)
	m.ReplyMarkup = numericKeyboard // todo: added keyboard in message send
	resp, err := tb.bot.Send(m)
	if err != nil {
		if err.Error() == errBlockedByUser {
			tb.blockUser(chatID)
			return nil, nil
		}
		return nil, fmt.Errorf("tb.bot.Send: %w", err)
	}

	return &resp, nil
}

func (tb *TBot) blockUser(chatID int64) error {
	return tb.getter.UpdChatActivity(chatID, false)
}

// func (t *TBot) SendStat(chatID int64, args string) error {
// 	if strings.TrimSpace(args) != "" {
// 		ss := strings.Split(args, " ")
// 		token := ss[0]
// 		if s.cfg.AccessToken != token {
// 			return errors.New("failed token")
// 		}
// 		count, err := s.cfg.Storage.GetCountUser()
// 		if err != nil {
// 			return err
// 		}
// 		s.Send(chatID, fmt.Sprintf("Активных пользователей в базе: %d", count))
// 	}
// 	return nil
// }
