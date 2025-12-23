package storage

import (
	"fmt"
	"time"

	"github.com/wenkaler/xfreehack/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/go-kit/kit/log"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type Storage struct {
	db     *sqlx.DB
	logger log.Logger
}

func New(dsn string, logger log.Logger) (*Storage, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn was empty")
	}
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}
	s := &Storage{
		db:     db,
		logger: logger,
	}

	return s, nil
}

// SaveCategory inserts or updates a category and returns its ID
func (s *Storage) SaveCategory(c model.Category) (int, error) {
	var id int
	err := s.db.QueryRow(`
		INSERT INTO categories (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, c.Name, c.Slug).Scan(&id)
	return id, err
}

// SaveStore inserts or updates a store and returns its ID
func (s *Storage) SaveStore(st model.Store) (int, error) {
	var id int
	err := s.db.QueryRow(`
		INSERT INTO stores (name, slug, url, category_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, url = EXCLUDED.url, category_id = EXCLUDED.category_id
		RETURNING id
	`, st.Name, st.Slug, st.URL, st.CategoryID).Scan(&id)
	return id, err
}

// SaveCoupon inserts a coupon
func (s *Storage) SaveCoupon(c model.Coupon) error {
	_, err := s.db.Exec(`
		INSERT INTO coupons (store_id, code, description, expiry_date, link, is_exclusive)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (link) DO UPDATE SET 
			code = EXCLUDED.code, 
			description = EXCLUDED.description, 
			expiry_date = EXCLUDED.expiry_date
	`, c.StoreID, c.Code, c.Description, c.ExpiryDate, c.Link, c.IsExclusive)
	return err
}

func (s *Storage) LoadCollect() (map[string]model.Coupon, error) {
	var m = make(map[string]model.Coupon)
	var rr []model.Coupon
	err := s.db.Select(&rr, `SELECT * FROM coupons`)
	if err != nil {
		return nil, err
	}
	for _, r := range rr {
		m[r.Link] = r
	}
	return m, nil
}

func (s *Storage) HasCollectionRunToday() (bool, error) {
	var count int
	err := s.db.Get(&count, `SELECT count(*) FROM collection_logs WHERE date(created_at) = date(now()) AND status = 'success'`)
	return count > 0, err
}

func (s *Storage) SaveCollectionLog(stats model.CollectionStats) error {
	status := "success"
	if !stats.Success {
		status = "failed"
	}
	_, err := s.db.Exec(`
		INSERT INTO collection_logs (status, categories_count, stores_count, coupons_count, duration_ms, error_message)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, status, stats.CategoriesCount, stats.StoresCount, stats.CouponsCount, stats.Duration.Milliseconds(), stats.ErrorMessage)
	return err
}

func (s *Storage) NewChat(chat *tgbotapi.Chat) error {
	_, err := s.db.Exec(`
		INSERT INTO chats (id, type, user_name, first_name, last_name, active)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET active = true
	`, chat.ID, chat.Type, chat.UserName, chat.FirstName, chat.LastName, true)
	return err
}

func (s *Storage) NewMessage(msg *tgbotapi.Message) error {
	_, err := s.db.Exec(`
		INSERT INTO messages (id, chat_id, message)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`, msg.MessageID, msg.Chat.ID, msg.Text)
	return err
}

func (s *Storage) SetNotificationSchedule(chatID int64, schedule string) error {
	_, err := s.db.Exec(`UPDATE chats SET notification_schedule = $1 WHERE id = $2`, schedule, chatID)
	return err
}

func (s *Storage) GetNotificationSchedule(chatID int64) (string, error) {
	var sched string
	err := s.db.Get(&sched, `SELECT notification_schedule FROM chats WHERE id = $1`, chatID)
	return sched, err
}

func (s *Storage) GetChatsByHour(utcHour int) ([]int64, error) {
	var chats []int64
	// Logic: We want users where (utcHour + timezone_offset) % 24 == notification_hour
	// PostgreSQL: MOD((utcHour + timezone_offset + 24), 24) = notification_hour
	// The +24 is to handle negative offsets correctly if modulo in DB behaves differently (though Postgres % is usually fine but +24 ensures positivity)
	query := `SELECT id FROM chats WHERE active = TRUE AND MOD(($1 + timezone_offset + 24), 24) = notification_hour`
	err := s.db.Select(&chats, query, utcHour)
	return chats, err
}

func (s *Storage) SetUserTimezone(chatID int64, offset int) error {
	_, err := s.db.Exec(`UPDATE chats SET timezone_offset = $1 WHERE id = $2`, offset, chatID)
	return err
}

func (s *Storage) SetUserNotificationHour(chatID int64, hour int) error {
	_, err := s.db.Exec(`UPDATE chats SET notification_hour = $1 WHERE id = $2`, hour, chatID)
	return err
}

func (s *Storage) GetUserTimezone(chatID int64) (int, error) {
	var off int
	err := s.db.Get(&off, `SELECT timezone_offset FROM chats WHERE id = $1`, chatID)
	return off, err
}

func (s *Storage) GetUserNotificationHour(chatID int64) (int, error) {
	var h int
	err := s.db.Get(&h, `SELECT notification_hour FROM chats WHERE id = $1`, chatID)
	return h, err
}

// SubscribeToCategory subscribes a user to a category
func (s *Storage) SubscribeToCategory(chatID int64, categoryID int) error {
	_, err := s.db.Exec(`
		INSERT INTO subscriptions (chat_id, category_id)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, category_id, store_id) DO NOTHING
	`, chatID, categoryID)
	return err
}

// UnsubscribeFromCategory unsubscribes a user from a category
func (s *Storage) UnsubscribeFromCategory(chatID int64, categoryID int) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE chat_id = $1 AND category_id = $2`, chatID, categoryID)
	return err
}

// SubscribeToStore subscribes a user to a store
func (s *Storage) SubscribeToStore(chatID int64, storeID int) error {
	_, err := s.db.Exec(`
		INSERT INTO subscriptions (chat_id, store_id)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, category_id, store_id) DO NOTHING
	`, chatID, storeID)
	return err
}

// UnsubscribeFromStore unsubscribes a user from a store
func (s *Storage) UnsubscribeFromStore(chatID int64, storeID int) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE chat_id = $1 AND store_id = $2`, chatID, storeID)
	return err
}

// GetSubscribedCategories returns IDs of categories the user is subscribed to
func (s *Storage) GetSubscribedCategories(chatID int64) ([]int, error) {
	var ids []int
	err := s.db.Select(&ids, `SELECT category_id FROM subscriptions WHERE chat_id = $1 AND category_id IS NOT NULL`, chatID)
	return ids, err
}

// GetSubscribedStores returns IDs of stores the user is subscribed to
func (s *Storage) GetSubscribedStores(chatID int64) ([]int, error) {
	var ids []int
	err := s.db.Select(&ids, `SELECT store_id FROM subscriptions WHERE chat_id = $1 AND store_id IS NOT NULL`, chatID)
	return ids, err
}

// GetAllCategories returns all available categories
func (s *Storage) GetAllCategories() ([]model.Category, error) {
	var cats []model.Category
	err := s.db.Select(&cats, `SELECT * FROM categories ORDER BY name`)
	return cats, err
}

// GetAllStores returns all available stores
func (s *Storage) GetAllStores() ([]model.Store, error) {
	var stores []model.Store
	err := s.db.Select(&stores, `SELECT * FROM stores ORDER BY name`)
	return stores, err
}

func (s *Storage) GetStoresByCategory(categoryID int) ([]model.Store, error) {
	var stores []model.Store
	err := s.db.Select(&stores, `SELECT * FROM stores WHERE category_id = $1 ORDER BY name`, categoryID)
	return stores, err
}

func (s *Storage) GetStore(storeID int) (*model.Store, error) {
	var st model.Store
	err := s.db.Get(&st, `SELECT * FROM stores WHERE id = $1`, storeID)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Storage) GetNotUseCoupon(cid int64) ([]model.Coupon, error) {
	return s.GetNotUseCouponCount(cid, 5)
}

func (s *Storage) GetNotUseCouponCount(cid, count int64) ([]model.Coupon, error) {
	var rr []model.Coupon
	var t = time.Now().AddDate(0, 0, -1).Unix()

	// Check if user has any subscriptions
	var subCount int
	err := s.db.Get(&subCount, `SELECT count(*) FROM subscriptions WHERE chat_id = $1`, cid)
	if err != nil {
		return nil, err
	}

	var query string
	var args []interface{}

	if subCount > 0 {
		// Filter by subscriptions (Category OR Store)
		query = `
			SELECT DISTINCT c.*
			FROM coupons c
			LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
			LEFT JOIN stores s ON c.store_id = s.id
			WHERE (rcc.status = FALSE OR rcc.status IS NULL) 
			  AND c.expiry_date > $2
			  AND (
			      s.category_id IN (SELECT category_id FROM subscriptions WHERE chat_id = $1 AND category_id IS NOT NULL)
			      OR
			      c.store_id IN (SELECT store_id FROM subscriptions WHERE chat_id = $1 AND store_id IS NOT NULL)
			  )
			LIMIT $3
		`
		args = []interface{}{cid, t, count}
	} else {
		// No subscriptions -> Return ALL (Legacy behavior)
		query = `
			SELECT c.*
			FROM coupons c
			LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
			WHERE (rcc.status = FALSE OR rcc.status IS NULL) AND c.expiry_date > $2
			LIMIT $3
		`
		args = []interface{}{cid, t, count}
	}

	err = s.db.Select(&rr, query, args...)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (s *Storage) GetUnsentNotification() ([]model.Notification, error) {
	var rr []model.Notification
	err := s.db.Select(&rr, `SELECT * FROM notifications WHERE sent = false`)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (s *Storage) MarkSentNotification(id int64) error {
	_, err := s.db.Exec(`UPDATE notifications SET sent = true WHERE id = $1`, id)
	return err
}

// GetCategoriesWithCoupons returns categories that have at least one store with active coupons
func (s *Storage) GetCategoriesWithCoupons() ([]model.Category, error) {
	var cats []model.Category
	query := `
		SELECT c.id, c.name, c.slug, COUNT(cp.id) as count
		FROM categories c
		JOIN stores s ON s.category_id = c.id
		JOIN coupons cp ON cp.store_id = s.id
		WHERE cp.expiry_date > $1
		GROUP BY c.id, c.name, c.slug
		HAVING COUNT(cp.id) > 0
		ORDER BY c.name
	`
	err := s.db.Select(&cats, query, time.Now().Unix())
	return cats, err
}

// GetStoresWithCoupons returns stores in a category that have active coupons
func (s *Storage) GetStoresWithCoupons(categoryID int) ([]model.Store, error) {
	var stores []model.Store
	query := `
		SELECT s.id, s.category_id, s.name, s.slug, s.url, COUNT(cp.id) as count
		FROM stores s
		JOIN coupons cp ON cp.store_id = s.id
		WHERE s.category_id = $1 AND cp.expiry_date > $2
		GROUP BY s.id, s.category_id, s.name, s.slug, s.url
		HAVING COUNT(cp.id) > 0
		ORDER BY s.name
	`
	err := s.db.Select(&stores, query, categoryID, time.Now().Unix())
	return stores, err
}

func (s *Storage) GetStoreCoupons(chatID int64, storeID int, count int64) ([]model.Coupon, error) {
	var rr []model.Coupon
	var t = time.Now().AddDate(0, 0, -1).Unix()
	query := `
		SELECT c.* FROM coupons c
		LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
		WHERE c.store_id = $2 AND c.expiry_date > $3
		  AND (rcc.status = FALSE OR rcc.status IS NULL)
		LIMIT $4
	`
	err := s.db.Select(&rr, query, chatID, storeID, t, count)
	return rr, err
}

func (s *Storage) CountNotUseCoupon(cid int64) (uint64, error) {
	var cnt uint64
	var t = time.Now().AddDate(0, 0, -1).Unix()

	var subCount int
	err := s.db.Get(&subCount, `SELECT count(*) FROM subscriptions WHERE chat_id = $1`, cid)
	if err != nil {
		return 0, err
	}

	var query string
	var args []interface{}

	if subCount > 0 {
		query = `
			SELECT count(DISTINCT c.id)
			FROM coupons c
			LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
			LEFT JOIN stores s ON c.store_id = s.id
			WHERE (rcc.status = FALSE OR rcc.status IS NULL) 
			  AND c.expiry_date > $2
			  AND (
			      s.category_id IN (SELECT category_id FROM subscriptions WHERE chat_id = $1 AND category_id IS NOT NULL)
			      OR
			      c.store_id IN (SELECT store_id FROM subscriptions WHERE chat_id = $1 AND store_id IS NOT NULL)
			  )
		`
		args = []interface{}{cid, t}
	} else {
		query = `
			SELECT count(c.id)
			FROM coupons c
			LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
			WHERE (rcc.status = FALSE OR rcc.status IS NULL) AND c.expiry_date > $2
		`
		args = []interface{}{cid, t}
	}

	err = s.db.Get(&cnt, query, args...)
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (s *Storage) CountNotUseCouponByStore(chatID int64, storeID int) (int, error) {
	var cnt int
	var t = time.Now().AddDate(0, 0, -1).Unix()
	// Count coupons for this store that are valid AND not marked as read (in relation_chat_coupons with status=true)
	// Note: status=FALSE or NULL means unread. status=TRUE means read.
	query := `
		SELECT count(c.id)
		FROM coupons c
		LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
		WHERE c.store_id = $2 AND c.expiry_date > $3
		  AND (rcc.status = FALSE OR rcc.status IS NULL)
	`
	err := s.db.Get(&cnt, query, chatID, storeID, t)
	return cnt, err
}

func (s *Storage) MarkAsRead(cid int64, rr []model.Coupon) error {
	for _, r := range rr {
		_, err := s.db.Exec(`
			INSERT INTO relation_chat_coupons (coupon_id, chat_id, status)
			VALUES ($1, $2, $3)
			ON CONFLICT (coupon_id, chat_id) DO UPDATE SET status = EXCLUDED.status
		`, r.ID, cid, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetChat() (a []int64, err error) {
	err = s.db.Select(&a, `SELECT id FROM chats WHERE active = true`)
	return
}

func (s *Storage) GetChatSettings(chatID int64) (*model.Chat, error) {
	var chat model.Chat
	err := s.db.Get(&chat, `SELECT * FROM chats WHERE id = $1`, chatID)
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

func (s *Storage) GetPendingNotifications() ([]model.Notification, error) {
	var n []model.Notification
	err := s.db.Select(&n, "SELECT id, message, target_segment, is_sent FROM notifications WHERE is_sent = false")
	return n, err
}

func (s *Storage) MarkNotificationSent(id int) error {
	_, err := s.db.Exec("UPDATE notifications SET is_sent = true, sent_at = NOW() WHERE id = $1", id)
	return err
}

func (s *Storage) GetChatsBySchedule(schedule string) (a []int64, err error) {
	err = s.db.Select(&a, `SELECT id FROM chats WHERE active = true AND notification_schedule = $1`, schedule)
	return
}

func (s *Storage) GetCountUser() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT count(id) FROM chats WHERE active = true`)
	return count, err
}

func (s *Storage) UpdChatActivity(cid int64, act bool) error {
	_, err := s.db.Exec(`UPDATE chats SET active = $1 WHERE id = $2`, act, cid)
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}
