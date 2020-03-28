package storage

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	"github.com/go-kit/kit/log/level"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wenkaler/xfreehack/collector"
)

type Storage struct {
	db     *sqlx.DB
	logger log.Logger
}

func New(pathDB string, logger log.Logger) (*Storage, error) {
	if pathDB == "" {
		return nil, fmt.Errorf("pathDB was empty")
	}
	db, err := sqlx.Open("sqlite3", pathDB)
	if err != nil {
		return nil, err
	}
	s := &Storage{
		db:     db,
		logger: logger,
	}
	err = s.init()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) Collect(record collector.Record) error {
	_, err := s.db.Exec(`INSERT INTO records(post_id, link, code, description, date) VALUES(?,?,?,?,?) ON CONFLICT(link) DO NOTHING`, record.PostID, record.Link, record.Code, record.Description, record.Date)
	return err
}

func (s *Storage) LoadCollect() (map[string]collector.Record, error) {
	var m = make(map[string]collector.Record)
	var rr []collector.Record
	err := s.db.Unsafe().Select(&rr, `SELECT * FROM records`)
	if err != nil {
		return nil, err
	}
	for _, r := range rr {
		m[r.PostID] = r
	}
	return m, nil
}

func (s *Storage) NewChat(chat *tgbotapi.Chat) error {
	_, err := s.db.Unsafe().Exec(`INSERT INTO chats(id, type, user_name, first_name, last_name, active) VALUES(?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, chat.ID, chat.Type, chat.UserName, chat.FirstName, chat.LastName, true)
	return err
}

func (s *Storage) NewMessage(msg *tgbotapi.Message) error {
	_, err := s.db.Unsafe().Exec(`INSERT INTO main.messages(id, id_chat, message) VALUES(?, ?, ?) ON CONFLICT(id) DO NOTHING`, msg.MessageID, msg.Chat.ID, msg.Text)
	return err
}

func (s *Storage) GetNotUseCoupon(cid int64) ([]collector.Record, error) {
	var rr []collector.Record
	var t = time.Now().AddDate(0, 0, -1).Unix()
	err := s.db.Unsafe().Select(&rr, `select records.* from records LEFT OUTER JOIN (SELECT * FROM relation_chat_records as rcr where rcr.id_chat = ?)  rcr on records.id = rcr.id_record where rcr.status = 0 and records.date = ? or rcr.id_record is null and records.date > ? limit 5`, cid, t, t)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (s *Storage) MarkAsRead(cid int64, rr []collector.Record) error {
	for _, r := range rr {
		_, err := s.db.Unsafe().Exec(`INSERT INTO relation_chat_records (id_record, id_chat, status) VALUES(?, ?, ?) ON CONFLICT(id_chat, id_record) DO UPDATE SET status = EXCLUDED.status`, r.ID, cid, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetChat() (a []int64, err error) {
	err = s.db.Unsafe().Select(&a, `SELECT id FROM chats WHERE active = 1`)
	return
}

func (s *Storage) UpdChatActivity(cid int64, act bool) error {
	_, err := s.db.Unsafe().Exec(`UPDATE chats SET active = ? where id = ?`, act, cid)
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) init() error {
	_, err := s.db.Unsafe().Exec(`CREATE TABLE  IF NOT EXISTS records(
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									post_id VARCHAR(40) NOT NULL,
									link VARCHAR(225) NOT NULL UNIQUE,
									code VARCHAR(100) NOT NULL,
									description TEXT NOT NULL,
									'date' BIGINT NOT NULL
						)`)
	if err != nil {
		return fmt.Errorf("failed create records table: %v", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS chats(
									id INTEGER PRIMARY KEY UNIQUE,
									'type' VARCHAR(225) NOT NULL,
									user_name VARCHAR(100) NULL,
									first_name VARCHAR(100) NULL,
									last_name VARCHAR(100) NULL,
									active BOOLEAN DEFAULT 1
						)`)
	if err != nil {
		return fmt.Errorf("failed create chats table: %v", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS messages(
									id INTEGER PRIMARY KEY UNIQUE,
									id_chat INTEGER NOT NULL,
									message TEXT NOT NULL,
									FOREIGN KEY (id_chat) REFERENCES chats(id)
						)`)
	if err != nil {
		return fmt.Errorf("failed create messages table: %v", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS relation_chat_records(
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									id_record INTEGER NOT NULL,
									id_chat INTEGER NOT NULL,
									status BOOLEAN DEFAULT FALSE ,
									FOREIGN KEY (id_chat) REFERENCES chats(id),
									FOREIGN KEY (id_record) REFERENCES records(id)
						)`)
	if err != nil {
		return fmt.Errorf("failed create messages table: %v", err)
	}

	_, err = s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS  rcr ON relation_chat_records(id_record, id_chat)`)
	if err != nil {
		return fmt.Errorf("failed create index table: %v", err)
	}
	level.Info(s.logger).Log("msg", "create data base, with table.")
	return nil
}
