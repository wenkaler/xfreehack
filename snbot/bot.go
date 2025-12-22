package snbot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wenkaler/xfreehack/model"

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
	GetNotUseCoupon(cid int64) ([]model.Coupon, error)
	GetNotUseCouponCount(cid, count int64) ([]model.Coupon, error)
	GetCountUser() (int, error)
	CountNotUseCoupon(cid int64) (uint64, error)
	MarkAsRead(cid int64, rr []model.Coupon) error
	NewChat(chat *tgbotapi.Chat) error
	UpdChatActivity(cid int64, act bool) error

	// Subscriptions
	SubscribeToCategory(chatID int64, categoryID int) error
	UnsubscribeFromCategory(chatID int64, categoryID int) error
	SubscribeToStore(chatID int64, storeID int) error
	UnsubscribeFromStore(chatID int64, storeID int) error
	GetSubscribedCategories(chatID int64) ([]int, error)
	GetSubscribedStores(chatID int64) ([]int, error)
	GetAllCategories() ([]model.Category, error)
	GetAllStores() ([]model.Store, error)
	SetNotificationSchedule(chatID int64, schedule string) error
	GetNotificationSchedule(chatID int64) (string, error)
	GetChatsBySchedule(schedule string) ([]int64, error)
	GetChat() ([]int64, error)
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
		code := rec.Code
		if code == "[автокод]" {
			code = "Не требуется (автоматически)"
		}
		msg = fmt.Sprintf("%v%v:\t%s \nКод--->: %s\nВремя истечения: %v\nОписание: %s\n\n", msg, i+1, rec.Link, code, time.Unix(rec.ExpiryDate, 0).Format("02.01.2006"), rec.Description)
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
	case "settings":
		s.sendSettingsMenu(message.Chat.ID)
	default:
		msg = info
		s.Send(message.Chat.ID, msg)
	}
	return nil
}

func (s *SNBot) Run() {
	for u := range s.upd {
		if u.CallbackQuery != nil {
			s.handleCallback(u.CallbackQuery)
			continue
		}
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

func (s *SNBot) handleCallback(cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data

	switch {
	case data == "settings_main":
		s.sendSettingsMenu(chatID)
	case data == "settings_cats":
		s.sendCategoriesMenu(chatID)
	case data == "settings_stores":
		s.sendStoresMenu(chatID)
	case strings.HasPrefix(data, "sub_cat_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "sub_cat_"))
		s.toggleCategorySubscription(chatID, id)
		s.sendCategoriesMenu(chatID) // refresh
	case strings.HasPrefix(data, "sub_store_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "sub_store_"))
		s.toggleStoreSubscription(chatID, id)
		s.sendStoresMenu(chatID) // refresh
	case data == "settings_time":
		s.sendTimeMenu(chatID)
	case strings.HasPrefix(data, "set_time_"):
		schedule := strings.TrimPrefix(data, "set_time_")
		s.cfg.Storage.SetNotificationSchedule(chatID, schedule)
		s.sendTimeMenu(chatID) // refresh
	}

	// Answer callback to stop loading animation
	s.bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}

func (s *SNBot) sendSettingsMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "⚙️ Настройки бота\nВыберите, что хотите настроить:")
	kbd := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗂 Категории", "settings_cats"),
			tgbotapi.NewInlineKeyboardButtonData("🏪 Магазины", "settings_stores"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Время уведомлений", "settings_time"),
		),
	)
	msg.ReplyMarkup = kbd
	s.bot.Send(msg)
}

func (s *SNBot) sendCategoriesMenu(chatID int64) {
	cats, err := s.cfg.Storage.GetAllCategories()
	if err != nil {
		s.Send(chatID, "Ошибка получения категорий")
		return
	}
	subs, err := s.cfg.Storage.GetSubscribedCategories(chatID)
	if err != nil {
		s.Send(chatID, "Ошибка получения подписок")
		return
	}

	subMap := make(map[int]bool)
	for _, id := range subs {
		subMap[id] = true
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, cat := range cats {
		label := cat.Name
		if subMap[cat.ID] {
			label = "✅ " + label
		} else {
			label = "❌ " + label
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("sub_cat_%d", cat.ID))
		row = append(row, btn)

		if (i+1)%2 == 0 || i == len(cats)-1 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите категории для подписки:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendStoresMenu(chatID int64) {
	stores, err := s.cfg.Storage.GetAllStores()
	if err != nil {
		s.Send(chatID, "Ошибка получения магазинов")
		return
	}
	subs, err := s.cfg.Storage.GetSubscribedStores(chatID)
	if err != nil {
		s.Send(chatID, "Ошибка получения подписок")
		return
	}

	subMap := make(map[int]bool)
	for _, id := range subs {
		subMap[id] = true
	}

	// For stores, list might be long. Ideally pagination, but for now simple list.
	// Limit to top 50 to avoid hitting limits? Or just show all if small.
	// Let's assume < 100 stores for now.

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, st := range stores {
		label := st.Name
		if subMap[st.ID] {
			label = "✅ " + label
		} else {
			label = "❌ " + label
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("sub_store_%d", st.ID))
		row = append(row, btn)

		if (i+1)%2 == 0 || i == len(stores)-1 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите магазины для подписки:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) toggleCategorySubscription(chatID int64, catID int) {
	subs, _ := s.cfg.Storage.GetSubscribedCategories(chatID)
	isSub := false
	for _, id := range subs {
		if id == catID {
			isSub = true
			break
		}
	}
	if isSub {
		s.cfg.Storage.UnsubscribeFromCategory(chatID, catID)
	} else {
		s.cfg.Storage.SubscribeToCategory(chatID, catID)
	}
}

func (s *SNBot) toggleStoreSubscription(chatID int64, storeID int) {
	subs, _ := s.cfg.Storage.GetSubscribedStores(chatID)
	isSub := false
	for _, id := range subs {
		if id == storeID {
			isSub = true
			break
		}
	}
	if isSub {
		s.cfg.Storage.UnsubscribeFromStore(chatID, storeID)
	} else {
		s.cfg.Storage.SubscribeToStore(chatID, storeID)
	}
}

func (s *SNBot) sendTimeMenu(chatID int64) {
	sched, err := s.cfg.Storage.GetNotificationSchedule(chatID)
	if err != nil {
		s.Send(chatID, "Ошибка получения настроек")
		return
	}

	labelImm := "Сразу при поступлении"
	label18 := "Каждый день в 18:00"

	if sched == "immediate" {
		labelImm = "✅ " + labelImm
		label18 = "❌ " + label18
	} else {
		labelImm = "❌ " + labelImm
		label18 = "✅ " + label18
	}

	kbd := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(labelImm, "set_time_immediate"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label18, "set_time_18:00"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите режим получения уведомлений:")
	msg.ReplyMarkup = kbd
	s.bot.Send(msg)
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
