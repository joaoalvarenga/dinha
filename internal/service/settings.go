package service

import (
	"database/sql"
	"strconv"
)

const defaultDaemonIntervalHours = 1

// GetDaemonIntervalHours returns the configured daemon scan interval in hours (1-12).
func GetDaemonIntervalHours(db *sql.DB) int {
	var val string
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'daemon_interval_hours'").Scan(&val)
	if err != nil {
		return defaultDaemonIntervalHours
	}
	hours, err := strconv.Atoi(val)
	if err != nil || hours < 1 || hours > 12 {
		return defaultDaemonIntervalHours
	}
	return hours
}

// SetDaemonIntervalHours saves the daemon scan interval (clamped to 1-12).
func SetDaemonIntervalHours(db *sql.DB, hours int) error {
	if hours < 1 {
		hours = 1
	}
	if hours > 12 {
		hours = 12
	}
	_, err := db.Exec(
		"INSERT INTO settings (key, value) VALUES ('daemon_interval_hours', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		strconv.Itoa(hours),
	)
	return err
}
