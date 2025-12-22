package storage

import (
	"fmt"
	"time"

	"github.com/wenkaler/xfreehack/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

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

func (s *Storage) GetNotUseCoupon(cid int64) ([]model.Coupon, error) {
	var rr []model.Coupon
	var t = time.Now().AddDate(0, 0, -1).Unix()
	err := s.db.Select(&rr, `
		SELECT c.*
		FROM coupons c
		LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
		WHERE (rcc.status = FALSE OR rcc.status IS NULL) AND c.expiry_date > $2
		LIMIT 5
	`, cid, t)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (s *Storage) GetNotUseCouponCount(cid, count int64) ([]model.Coupon, error) {
	var rr []model.Coupon
	var t = time.Now().AddDate(0, 0, -1).Unix()
	err := s.db.Select(&rr, `
		SELECT c.*
		FROM coupons c
		LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
		WHERE (rcc.status = FALSE OR rcc.status IS NULL) AND c.expiry_date > $2
		LIMIT $3
	`, cid, t, count)
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

func (s *Storage) CountNotUseCoupon(cid int64) (uint64, error) {
	var cnt uint64
	var t = time.Now().AddDate(0, 0, -1).Unix()
	err := s.db.Get(&cnt, `
		SELECT count(c.id)
		FROM coupons c
		LEFT JOIN relation_chat_coupons rcc ON c.id = rcc.coupon_id AND rcc.chat_id = $1
		WHERE (rcc.status = FALSE OR rcc.status IS NULL) AND c.expiry_date > $2
	`, cid, t)
	if err != nil {
		return 0, err
	}
	return cnt, nil
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
