package service

import (
	"database/sql"
	"log"
	"time"

	"github.com/joaoalvarenga/dinha/internal/model"
)

func ListWatches(db *sql.DB) ([]model.Watch, error) {
	rows, err := db.Query("SELECT absolute_file_path, created_at, default_expiration FROM watches")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var watches []model.Watch
	for rows.Next() {
		var w model.Watch
		if err := rows.Scan(&w.AbsoluteFilePath, &w.CreatedAt, &w.DefaultExpiration); err != nil {
			return nil, err
		}
		watches = append(watches, w)
	}
	return watches, rows.Err()
}

func FindWatch(db *sql.DB, path string) (*model.Watch, error) {
	row := db.QueryRow("SELECT absolute_file_path, created_at, default_expiration FROM watches WHERE absolute_file_path = ?", path)

	var w model.Watch
	err := row.Scan(&w.AbsoluteFilePath, &w.CreatedAt, &w.DefaultExpiration)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func UpsertWatch(db *sql.DB, path string, expiration *int32) {
	existing, err := FindWatch(db, path)
	if err != nil {
		log.Printf("error finding watch: %v", err)
		return
	}

	if existing != nil {
		_, err := db.Exec(
			"UPDATE watches SET default_expiration = ? WHERE absolute_file_path = ?",
			expiration, existing.AbsoluteFilePath,
		)
		if err != nil {
			log.Printf("error updating watch: %v", err)
		}
		return
	}

	now := time.Now()
	_, err = db.Exec(
		"INSERT INTO watches (absolute_file_path, created_at, default_expiration) VALUES (?, ?, ?)",
		path, now, expiration,
	)
	if err != nil {
		log.Printf("error inserting watch: %v", err)
	}
}

func DeleteWatch(db *sql.DB, path string) {
	_, err := db.Exec("DELETE FROM files_watches WHERE watch_id = ?", path)
	if err != nil {
		log.Printf("error deleting files_watches: %v", err)
		return
	}

	_, err = db.Exec("DELETE FROM watches WHERE absolute_file_path = ?", path)
	if err != nil {
		log.Printf("error deleting watch: %v", err)
	}
}
