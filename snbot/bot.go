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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	GetCategoriesWithCoupons() ([]model.Category, error)
	GetAllStores() ([]model.Store, error)
	GetStoresWithCoupons(categoryID int) ([]model.Store, error)
	GetStoresByCategory(categoryID int) ([]model.Store, error)
	GetStore(storeID int) (*model.Store, error)
	GetStoreCoupons(chatID int64, storeID int, count int64) ([]model.Coupon, error)
	SetNotificationSchedule(chatID int64, schedule string) error
	GetNotificationSchedule(chatID int64) (string, error)
	GetChatsBySchedule(schedule string) ([]int64, error)
	GetChatsByHour(utcHour int) ([]int64, error)          // New
	SetUserTimezone(chatID int64, offset int) error       // New
	SetUserNotificationHour(chatID int64, hour int) error // New
	GetUserTimezone(chatID int64) (int, error)            // New (offset)
	GetUserNotificationHour(chatID int64) (int, error)    // New
	GetChat() ([]int64, error)

	// Notifications
	GetPendingNotifications() ([]model.Notification, error)

	MarkNotificationSent(id int) error
	CountNotUseCouponByStore(chatID int64, storeID int) (int, error)
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
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Запустить бота 🚀"},
		{Command: "print", Description: "Вывести купоны 🏷️"},
		{Command: "settings", Description: "Настройки ⚙️"},
		{Command: "donate", Description: "Поддержать автора ☕️"},
	}
	// Native SetMyCommands in v5
	if _, err := bot.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		level.Error(cfg.Logger).Log("msg", "Failed to set bot commands", "err", err)
	}

	level.Info(cfg.Logger).Log("msg", "Authorized on account", "bot-name", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = cfg.UpdateTime
	updates := bot.GetUpdatesChan(u)
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
	case "donate":
		s.sendDonate(message.Chat.ID)
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

	// --- My Subscriptions ---
	case data == "settings_my_subs":
		s.sendMySubscriptions(chatID)
	case strings.HasPrefix(data, "unsub_cat_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "unsub_cat_"))
		s.cfg.Storage.UnsubscribeFromCategory(chatID, id)
		s.sendMySubscriptions(chatID)
	case strings.HasPrefix(data, "unsub_store_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "unsub_store_"))
		s.cfg.Storage.UnsubscribeFromStore(chatID, id)
		s.sendMySubscriptions(chatID)

	// --- Add Subscription ---
	case data == "settings_add_sub":
		s.sendAddSubscriptionMenu(chatID)
	case data == "add_sub_cat":
		s.sendAddSubCategories(chatID)
	case strings.HasPrefix(data, "do_sub_cat_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "do_sub_cat_"))
		s.cfg.Storage.SubscribeToCategory(chatID, id)
		s.sendAddSubCategories(chatID) // refresh
	case data == "add_sub_store_cats":
		s.sendAddSubStoreCategories(chatID)
	case strings.HasPrefix(data, "add_sub_store_list_"):
		catID, _ := strconv.Atoi(strings.TrimPrefix(data, "add_sub_store_list_"))
		s.sendAddSubStores(chatID, catID)
	case strings.HasPrefix(data, "do_sub_store_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "do_sub_store_"))
		s.cfg.Storage.SubscribeToStore(chatID, id)
		// refresh store list, need catID. Retrieve store to get catID
		st, _ := s.cfg.Storage.GetStore(id)
		if st != nil {
			s.sendAddSubStores(chatID, st.CategoryID)
		}

	// --- Coupons Browser ---
	case data == "browser_main":
		s.sendCouponsBrowser(chatID)
	case strings.HasPrefix(data, "browser_cat_"):
		catID, _ := strconv.Atoi(strings.TrimPrefix(data, "browser_cat_"))
		s.sendCouponsBrowserStores(chatID, catID, 0) // 0 means new message
	case strings.HasPrefix(data, "browser_print_store_"):
		storeID, _ := strconv.Atoi(strings.TrimPrefix(data, "browser_print_store_"))
		s.printStoreCoupons(cb, storeID) // Pass full callback query

	// --- Time Settings ---
	case data == "settings_time":
		s.sendTimeMenu(chatID)
	case data == "time_set_timezone":
		s.sendTimezoneMenu(chatID)
	case strings.HasPrefix(data, "set_tz_"):
		tz, _ := strconv.Atoi(strings.TrimPrefix(data, "set_tz_"))
		s.cfg.Storage.SetUserTimezone(chatID, tz)
		s.sendTimeMenu(chatID)
	case data == "time_set_hour":
		s.sendHourMenu(chatID)
	case strings.HasPrefix(data, "set_hour_"):
		hour, _ := strconv.Atoi(strings.TrimPrefix(data, "set_hour_"))
		s.cfg.Storage.SetUserNotificationHour(chatID, hour)
		s.sendTimeMenu(chatID)

	// --- Donation ---
	case data == "donate_action":
		s.sendDonate(chatID)
	}

	// Answer callback to stop loading animation
	if _, err := s.bot.Request(tgbotapi.NewCallback(cb.ID, "")); err != nil {
		level.Error(s.cfg.Logger).Log("msg", "failed to answer callback", "err", err)
	}
}

func (s *SNBot) sendSettingsMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "⚙️ Настройки бота")
	kbd := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐️ Мои подписки", "settings_my_subs"),
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить подписку", "settings_add_sub"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏷️ Браузер купонов", "browser_main"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Время уведомлений", "settings_time"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("☕️ Поддержать автора", "donate_action"),
		),
	)
	msg.ReplyMarkup = kbd
	s.bot.Send(msg)
}

func (s *SNBot) sendMySubscriptions(chatID int64) {
	catIds, _ := s.cfg.Storage.GetSubscribedCategories(chatID)
	storeIds, _ := s.cfg.Storage.GetSubscribedStores(chatID)
	catsAll, _ := s.cfg.Storage.GetAllCategories()
	storesAll, _ := s.cfg.Storage.GetAllStores()

	var rows [][]tgbotapi.InlineKeyboardButton

	// Categories
	for _, cid := range catIds {
		var name string
		for _, c := range catsAll {
			if c.ID == cid {
				name = c.Name
				break
			}
		}
		if name != "" {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➖ "+name, fmt.Sprintf("unsub_cat_%d", cid)),
			))
		}
	}

	// Stores
	for _, sid := range storeIds {
		var name string
		for _, st := range storesAll {
			if st.ID == sid {
				name = st.Name
				break
			}
		}
		if name != "" {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➖ "+name, fmt.Sprintf("unsub_store_%d", sid)),
			))
		}
	}

	if len(rows) == 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📭 У вас нет активных подписок", "noop"),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
	))

	msg := tgbotapi.NewMessage(chatID, "Ваши подписки (нажмите для отписки):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendAddSubscriptionMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "➕ Добавить подписку")
	kbd := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗂 Категории", "add_sub_cat"),
			tgbotapi.NewInlineKeyboardButtonData("🏪 Магазины", "add_sub_store_cats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
		),
	)
	msg.ReplyMarkup = kbd
	s.bot.Send(msg)
}

func (s *SNBot) sendAddSubCategories(chatID int64) {
	cats, _ := s.cfg.Storage.GetCategoriesWithCoupons()
	subs, _ := s.cfg.Storage.GetSubscribedCategories(chatID)
	subMap := make(map[int]bool)
	for _, id := range subs {
		subMap[id] = true
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	count := 0
	for _, cat := range cats {
		if subMap[cat.ID] {
			continue // Already subscribed
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✅ %s (%d)", cat.Name, cat.Count), fmt.Sprintf("do_sub_cat_%d", cat.ID))
		row = append(row, btn)
		count++
		if count%2 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_add_sub"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите категорию для подписки:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendAddSubStoreCategories(chatID int64) {
	cats, _ := s.cfg.Storage.GetCategoriesWithCoupons()
	// Filter: Don't show category if user is ALREADY subscribed to it (logic: if subbed to cat, don't need store sub)
	subs, _ := s.cfg.Storage.GetSubscribedCategories(chatID)
	subMap := make(map[int]bool)
	for _, id := range subs {
		subMap[id] = true
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	count := 0
	for _, cat := range cats {
		if subMap[cat.ID] {
			continue
		}
		btn := tgbotapi.NewInlineKeyboardButtonData("📂 "+cat.Name, fmt.Sprintf("add_sub_store_list_%d", cat.ID))
		row = append(row, btn)
		count++
		if count%2 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_add_sub"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите категорию магазина:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendAddSubStores(chatID int64, catID int) {
	stores, _ := s.cfg.Storage.GetStoresWithCoupons(catID) // Only stores with coupons
	subs, _ := s.cfg.Storage.GetSubscribedStores(chatID)
	subMap := make(map[int]bool)
	for _, id := range subs {
		subMap[id] = true
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	count := 0
	for _, st := range stores {
		if subMap[st.ID] {
			continue
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✅ %s (%d)", st.Name, st.Count), fmt.Sprintf("do_sub_store_%d", st.ID))
		row = append(row, btn)
		count++
		if count%2 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "add_sub_store_cats"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите магазин для подписки:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendCouponsBrowser(chatID int64) {
	cats, _ := s.cfg.Storage.GetCategoriesWithCoupons()

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	count := 0
	for _, cat := range cats {
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📂 %s (%d)", cat.Name, cat.Count), fmt.Sprintf("browser_cat_%d", cat.ID))
		row = append(row, btn)
		count++
		if count%2 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "settings_main"),
	))

	msg := tgbotapi.NewMessage(chatID, "🏷️ Браузер купонов (Категории):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendCouponsBrowserStores(chatID int64, catID int, editMessageID int) {
	// Logic: Get stores, BUT only those with coupons that are NOT read by this user?
	// The current GetStoresWithCoupons does NOT filter by "read by user". It just checks expiry.
	// We need a method that respects "unread by user".
	// However, user said "If coupons were shown we don't show them to the user anymore".
	// So we need GetStoresWithUnreadCoupons(categoryID, chatID).
	// Existing GetStoresWithCoupons is generic.
	// Let's modify GetStoresWithCoupons in storage or create a new one?
	// For now, let's use what we have, but we really should filter.
	// The User said: "If coupons were shown we don't show them to the user anymore." -> This implies the LIST should likely change counts.
	// Since we marked as read in printStoreCoupons, we need GetStoresWithCoupons to be user-aware or have a new method.
	// Let's assume for this step we update the signature first, and then I will update Storage to be user-aware.

	stores, _ := s.cfg.Storage.GetStoresWithCoupons(catID) // This needs update ideally

	// Better: We need to filter stores here or in SQL.
	// Let's leave SQL for next step and just do UI logic here.

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	count := 0
	for _, st := range stores {
		// We need to get accurate count for THIS user ideally.
		// If we use static count, it won't decrease.
		// Let's check coupon count for this user?
		// expensive loop?
		cnt, _ := s.cfg.Storage.CountNotUseCouponByStore(chatID, st.ID)
		if cnt == 0 {
			continue
		}

		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🏪 %s (%d)", st.Name, cnt), fmt.Sprintf("browser_print_store_%d", st.ID))
		row = append(row, btn)
		count++
		if count%2 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "browser_main"),
	))

	kbd := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if editMessageID > 0 {
		msg := tgbotapi.NewEditMessageTextAndMarkup(chatID, editMessageID, "Выберите магазин для просмотра купонов:", kbd)
		s.bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, "Выберите магазин для просмотра купонов:")
		msg.ReplyMarkup = kbd
		s.bot.Send(msg)
	}
}

func (s *SNBot) printStoreCoupons(cb *tgbotapi.CallbackQuery, storeID int) {
	chatID := cb.Message.Chat.ID
	messageID := cb.Message.MessageID

	store, _ := s.cfg.Storage.GetStore(storeID)
	// Fetch coupons (default 5)
	coupons, err := s.cfg.Storage.GetStoreCoupons(chatID, storeID, 5)
	if err != nil {
		s.Send(chatID, "Ошибка получения купонов")
		return
	}
	if len(coupons) == 0 {
		// Answer callback instead of sending message
		s.bot.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "Купоны не найдены"))
		// Refresh menu to remove empty store if needed
		s.sendCouponsBrowserStores(chatID, store.CategoryID, messageID) // Pass messageID to Edit
		return
	}

	var msg string
	msg = fmt.Sprintf("🏷️ Купоны магазина %s:\n\n", store.Name)
	for i, rec := range coupons {
		code := rec.Code
		if code == "[автокод]" {
			code = "Не требуется (автоматически)"
		}
		expiry := "Не указано"
		if rec.ExpiryDate > 0 {
			expiry = time.Unix(rec.ExpiryDate, 0).Format("02.01.2006")
		}
		msg += fmt.Sprintf("%d. %s\nКод: `%s`\nДействует до: %s\n\n", i+1, rec.Description, code, expiry)
		msg += fmt.Sprintf("🔗 [Перейти к скидке](%s)\n\n", rec.Link)
	}

	// 1. Send coupons as a new message (without buttons, or maybe just a delete button?)
	// User said "don't fall into menu with back button", implies they just want the content.
	m := tgbotapi.NewMessage(chatID, msg)
	m.ParseMode = "Markdown"
	s.bot.Send(m)

	// 2. Mark as read
	err = s.cfg.Storage.MarkAsRead(chatID, coupons)
	if err != nil {
		level.Error(s.cfg.Logger).Log("msg", "failed to mark as read", "err", err)
	}

	// 3. Refresh the Store List
	// User feedback: "After calling store, menu is not re-rendered (conveniently)".
	// Old logic: Edit message. Result: Menu stays "above" the new coupons.
	// New logic: Delete old menu message, Send NEW menu message at the bottom.

	// Delete old message
	s.bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID))

	// Send new menu (0 passed as editMessageID means NewMessage)
	s.sendCouponsBrowserStores(chatID, store.CategoryID, 0)
}

func (s *SNBot) sendTimeMenu(chatID int64) {
	tz, _ := s.cfg.Storage.GetUserTimezone(chatID)
	hour, _ := s.cfg.Storage.GetUserNotificationHour(chatID)

	// Display current settings
	// Timezone e.g. UTC+3
	// Hour e.g. 18:00
	msgText := fmt.Sprintf("⏰ Настройки уведомлений\n\n🌍 Ваш часовой пояс: UTC+%d\n🕒 Час получения: %02d:00 (по вашему времени)", tz, hour)

	kbd := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌍 Изменить часовой пояс", "time_set_timezone"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🕒 Изменить время", "time_set_hour"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ReplyMarkup = kbd
	s.bot.Send(msg)
}

func (s *SNBot) sendTimezoneMenu(chatID int64) {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Range UTC+2 to UTC+12
	for i := 2; i <= 12; i++ {
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("UTC+%d", i), fmt.Sprintf("set_tz_%d", i))
		row = append(row, btn)
		if len(row) == 3 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_time"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите ваш часовой пояс (например, Москва = UTC+3):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) sendHourMenu(chatID int64) {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Range 00 to 23
	for i := 0; i < 24; i++ {
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%02d:00", i), fmt.Sprintf("set_hour_%d", i))
		row = append(row, btn)
		if len(row) == 4 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_time"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выберите час получения уведомлений (по вашему времени):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.bot.Send(msg)
}

func (s *SNBot) Send(chatID int64, msg string) error {
	level.Error(s.cfg.Logger).Log("msg", "try send", "chatID", chatID)
	m := tgbotapi.NewMessage(chatID, msg)
	// Remove persistence keyboard logic again just in case, though we use NewRemoveKeyboard only when needed.
	// Actually we should NOT NewRemoveKeyboard always if we are in inline menu flow.
	// But standard message sending implies text response.
	// m.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true) // We removed persistent menu earlier.
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

func (s *SNBot) sendDonate(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Если бот оказался полезен, вы можете поддержать его развитие. Спасибо! ❤️\n\nВаша поддержка помогает оплачивать сервер и мотивирует добавлять новые функции.")

	// URL button
	btn := tgbotapi.NewInlineKeyboardButtonURL("☕️ Поддержать рублем", "https://pay.cloudtips.ru/p/b7865ee8")

	row1 := tgbotapi.NewInlineKeyboardRow(btn)
	row2 := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "settings_main"))

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row1, row2)
	s.bot.Send(msg)
}
