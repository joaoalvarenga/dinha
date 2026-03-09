package service

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/joaoalvarenga/dinha/internal/model"
)

func FindFile(db *sql.DB, path string) (*model.File, error) {
	row := db.QueryRow("SELECT absolute_file_path, inserted_at, modified_at, accessed_at, expiration FROM files WHERE absolute_file_path = ?", path)

	var f model.File
	var expiration sql.NullTime
	err := row.Scan(&f.AbsoluteFilePath, &f.InsertedAt, &f.ModifiedAt, &f.AccessedAt, &expiration)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.Expiration = expiration
	return &f, nil
}

// SyncFile creates or updates a file record.
// Expiration is computed from the most recent of (inserted_at, modified_at, accessed_at) + expirationSeconds.
// A file is only considered expired when ALL three dates are older than the threshold.
func SyncFile(db *sql.DB, path string, modTime, accessTime time.Time, expirationSeconds *int32) {
	existing, err := FindFile(db, path)
	if err != nil {
		log.Printf("error finding file: %v", err)
		return
	}

	now := time.Now()

	if existing != nil {
		// Only update if mod time or access time changed
		if !modTime.After(existing.ModifiedAt) && !accessTime.After(existing.AccessedAt) {
			return
		}
		log.Printf("File updated: %s", path)

		expiration := calcExpiration(modTime, accessTime, expirationSeconds)
		_, err := db.Exec(
			"UPDATE files SET modified_at = ?, accessed_at = ?, expiration = ? WHERE absolute_file_path = ?",
			modTime, accessTime, expiration, existing.AbsoluteFilePath,
		)
		if err != nil {
			log.Printf("error updating file: %v", err)
		}
		return
	}

	log.Printf("New file: %s", path)
	expiration := calcExpiration(modTime, accessTime, expirationSeconds)
	_, err = db.Exec(
		"INSERT INTO files (absolute_file_path, inserted_at, modified_at, accessed_at, expiration) VALUES (?, ?, ?, ?, ?)",
		path, now, modTime, accessTime, expiration,
	)
	if err != nil {
		log.Printf("error inserting file: %v", err)
	}
}

// calcExpiration returns the expiration timestamp based on the latest activity date.
// latest = max(insertedAt, modTime, accessTime), then expiration = latest + duration.
func calcExpiration(modTime, accessTime time.Time, expirationSeconds *int32) sql.NullTime {
	if expirationSeconds == nil {
		return sql.NullTime{}
	}

	latest := modTime
	if accessTime.After(latest) {
		latest = accessTime
	}

	return sql.NullTime{
		Time:  latest.Add(time.Duration(*expirationSeconds) * time.Second),
		Valid: true,
	}
}

// ListExpiredFiles returns files whose expiration has passed.
// Files that no longer exist on disk are automatically removed from the database.
func ListExpiredFiles(db *sql.DB, isDaemon bool) ([]model.File, error) {
	now := time.Now()
	rows, err := db.Query(
		"SELECT absolute_file_path, inserted_at, modified_at, accessed_at, expiration FROM files WHERE expiration IS NOT NULL AND expiration < ?",
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []model.File
	var stale []string
	for rows.Next() {
		var f model.File
		var expiration sql.NullTime
		if err := rows.Scan(&f.AbsoluteFilePath, &f.InsertedAt, &f.ModifiedAt, &f.AccessedAt, &expiration); err != nil {
			return nil, err
		}
		f.Expiration = expiration
		if isDaemon {
			if _, err := os.Stat(f.AbsoluteFilePath); os.IsNotExist(err) {
				stale = append(stale, f.AbsoluteFilePath)
				continue
			}
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if isDaemon {
		for _, path := range stale {
			removeFileRecord(db, path)
		}
	}

	return files, nil
}

func removeFileRecord(db *sql.DB, path string) {
	if _, err := db.Exec("DELETE FROM files_watches WHERE file_id = ?", path); err != nil {
		log.Printf("error cleaning up files_watches for %s: %v", path, err)
	}
	if _, err := db.Exec("DELETE FROM files WHERE absolute_file_path = ?", path); err != nil {
		log.Printf("error cleaning up stale file record %s: %v", path, err)
	}
}

// DeleteFile removes a file record from the database and deletes the file from disk.
func DeleteFile(db *sql.DB, path string) error {
	_, err := db.Exec("DELETE FROM files_watches WHERE file_id = ?", path)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM files WHERE absolute_file_path = ?", path)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// DeletePath removes a file or directory from disk and cleans up all related DB records.
func DeletePath(db *sql.DB, path string) error {
	// Remove DB records for the path itself and anything inside it (for directories)
	_, err := db.Exec("DELETE FROM files_watches WHERE file_id LIKE ? OR file_id = ?", path+"/%", path)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM files WHERE absolute_file_path LIKE ? OR absolute_file_path = ?", path+"/%", path)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}
