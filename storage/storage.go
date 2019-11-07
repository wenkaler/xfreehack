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

func (s *Storage) init() error {
	_, err := s.db.Exec(`CREATE TABLE  IF NOT EXISTS records(
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									post_id VARCHAR(40) NOT NULL,
									market VARCHAR(40) NOT NULL,
									link VARCHAR(225) NOT NULL,
									code VARCHAR(100) NOT NULL
						)`)
	if err != nil {
		return fmt.Errorf("failed create records table: %v", err)
	}
	level.Info(s.logger).Log("msg", "create data base, with table.")
	return nil
}
