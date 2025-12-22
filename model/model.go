package model

type Notification struct {
	ID      int64
	Message string
	Status  bool
}

type Category struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Slug string `db:"slug"`
}

type Store struct {
	ID         int    `db:"id"`
	CategoryID int    `db:"category_id"`
	Name       string `db:"name"`
	Slug       string `db:"slug"`
	URL        string `db:"url"`
}

type Coupon struct {
	ID          int64  `db:"id"`
	StoreID     int    `db:"store_id"`
	Code        string `db:"code"`
	Description string `db:"description"`
	ExpiryDate  int64  `db:"expiry_date"`
	Link        string `db:"link"`
	IsExclusive bool   `db:"is_exclusive"`
}
