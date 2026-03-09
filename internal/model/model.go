package model

import (
	"database/sql"
	"time"
)

type File struct {
	AbsoluteFilePath string
	InsertedAt       time.Time
	ModifiedAt       time.Time
	AccessedAt       time.Time
	Expiration       sql.NullTime
}

type Watch struct {
	AbsoluteFilePath  string
	CreatedAt         time.Time
	DefaultExpiration sql.NullInt32
}

type FileWatch struct {
	FileID  string
	WatchID string
}
