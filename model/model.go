package model

import "time"

type Notification struct {
	ID            int    `db:"id"`
	Message       string `db:"message"`
	TargetSegment string `db:"target_segment"`
	IsSent        bool   `db:"is_sent"`
}

type Category struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Slug  string `db:"slug"`
	Count int    `db:"count"`
}

type Store struct {
	ID         int    `db:"id"`
	CategoryID int    `db:"category_id"`
	Name       string `db:"name"`
	Slug       string `db:"slug"`
	URL        string `db:"url"`
	Count      int    `db:"count"`
}

type Coupon struct {
	ID          int64     `db:"id"`
	StoreID     int       `db:"store_id"`
	Code        string    `db:"code"`
	Description string    `db:"description"`
	ExpiryDate  int64     `db:"expiry_date"`
	Link        string    `db:"link"`
	IsExclusive bool      `db:"is_exclusive"`
	CreatedAt   time.Time `db:"created_at"`
}

type Subscription struct {
	ID         int   `db:"id"`
	ChatID     int64 `db:"chat_id"`
	CategoryID *int  `db:"category_id"`
	StoreID    *int  `db:"store_id"`
}

type Chat struct {
	ID                   int64     `db:"id"`
	Type                 string    `db:"type"`
	UserName             string    `db:"username"`
	FirstName            string    `db:"first_name"`
	LastName             string    `db:"last_name"`
	Active               bool      `db:"active"`
	NotificationSchedule string    `db:"notification_schedule"`
	TimezoneOffset       int       `db:"timezone_offset"`
	NotificationHour     int       `db:"notification_hour"`
	CreatedAt            time.Time `db:"created_at"`
}

type CollectionStats struct {
	CategoriesCount int
	StoresCount     int
	CouponsCount    int
	Duration        time.Duration
	Success         bool
	ErrorMessage    string
}
