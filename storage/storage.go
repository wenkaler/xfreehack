package storage

import (
	"fmt"

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
	_, err := s.db.Exec(`INSERT INTO records(post_id, market, link, code) VALUES(?,?,?,?)`, record.PostID, record.Market, record.Link, record.Code)
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

func (s *Storage) init() error {
	_, err := s.db.Unsafe().Exec(`CREATE TABLE  IF NOT EXISTS records(
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									post_id VARCHAR(40) NOT NULL UNIQUE,
									market VARCHAR(40) NOT NULL,
									link VARCHAR(225) NOT NULL,
									code VARCHAR(100) NOT NULL
						)`)
	if err != nil {
		return fmt.Errorf("failed create records table: %v", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS chats(
									id INTEGER PRIMARY KEY UNIQUE
						)`)
	if err != nil {
		return fmt.Errorf("failed create chats table: %v", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS messages(
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									chat_id INTEGER NOT NULL,
									message TEXT NOT NULL,
									FOREIGN KEY (chat_id) REFERENCES chats(id)
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
	level.Info(s.logger).Log("msg", "create data base, with table.")
	return nil
}

func (s *Storage) NewChat(cid int64) error {
	_, err := s.db.Unsafe().Exec(`INSERT INTO chats(id) VALUES(?)`, cid)
	return err
}

func (s *Storage) GetNotUseCoupon(cid int64) ([]collector.Record, error) {
	var rr []collector.Record
	err := s.db.Unsafe().Select(&rr, `SELECT rec.* FROM relation_chat_records AS rel WHERE id_chat = ? AND status = false INNER JOIN records AS rec ON rel.id_record = rec.id`, cid)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (s *Storage) MarkAsRead(cid int64, rr []collector.Record) error {
	for _, r := range rr {
		_, err := s.db.Unsafe().Exec(`UPDATE TABLE relation_chat_records SET status = true WHERE id_record = ? and id_chat = ?`, r.ID, cid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetChat() (a []int64, err error) {
	err = s.db.Unsafe().Select(&a, `SELECT * FROM chats`)
	return
}

func (s *Storage) Close() error {
	return s.db.Close()
}
