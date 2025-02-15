package model

type Notification struct {
	ID      int64
	Message string
	Status  bool
}

type Record struct {
	ID          string `db:"id"`
	Code        string `db:"code"`
	Date        int64  `db:"date"`
	Link        string `db:"link"`
	PostID      string `db:"post_id"`
	Description string `db:"description"`
}
