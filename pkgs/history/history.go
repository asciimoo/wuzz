// History package manages the storage and retrieval of request/response history using SQLite.
package history

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Entry represents a single request/response in history
type Entry struct {
	ID              int64
	Timestamp       time.Time
	URL             string
	Method          string
	GetParams       string
	Data            string
	Headers         string
	ResponseHeaders string
	RawResponseBody []byte
	ContentType     string
	Duration        time.Duration
}

// Package-level singleton variables
var (
	db          *sql.DB
	mu          sync.RWMutex
	initialized bool
)

// Init initializes the history database at the specified path
// This must be called before any other history functions
func Init(dbPath string) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return fmt.Errorf("history already initialized")
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		err := db.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		db.Close()
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	initialized = true
	return nil
}

// Close closes the database connection
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return nil
	}

	if db != nil {
		if err := db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		db = nil
	}

	initialized = false
	return nil
}

// AddEntry adds a new entry to the history
func AddEntry(entry *Entry) error {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return fmt.Errorf("history not initialized")
	}

	query := `
		INSERT INTO history (
			timestamp, url, method, get_params, data, headers,
			response_headers, raw_response_body, content_type, duration_ns
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		entry.Timestamp,
		entry.URL,
		entry.Method,
		entry.GetParams,
		entry.Data,
		entry.Headers,
		entry.ResponseHeaders,
		entry.RawResponseBody,
		entry.ContentType,
		entry.Duration.Nanoseconds(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert entry: %w", err)
	}

	// Update the entry ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	entry.ID = id

	return nil
}

// GetHistory retrieves all history entries, ordered by newest first
func GetHistory() ([]*Entry, error) {
	mu.RLock()
	defer mu.RUnlock()

	if !initialized {
		return nil, fmt.Errorf("history not initialized")
	}

	query := `
		SELECT id, timestamp, url, method, get_params, data, headers,
			response_headers, raw_response_body, content_type, duration_ns
		FROM history
		ORDER BY timestamp DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		entry := &Entry{}
		var durationNs int64

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.URL,
			&entry.Method,
			&entry.GetParams,
			&entry.Data,
			&entry.Headers,
			&entry.ResponseHeaders,
			&entry.RawResponseBody,
			&entry.ContentType,
			&durationNs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		entry.Duration = time.Duration(durationNs)
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// GetEntryByID retrieves a specific entry by its ID
func GetEntryByID(id int64) (*Entry, error) {
	mu.RLock()
	defer mu.RUnlock()

	if !initialized {
		return nil, fmt.Errorf("history not initialized")
	}

	query := `
		SELECT id, timestamp, url, method, get_params, data, headers,
			response_headers, raw_response_body, content_type, duration_ns
		FROM history
		WHERE id = ?
	`

	entry := &Entry{}
	var durationNs int64

	err := db.QueryRow(query, id).Scan(
		&entry.ID,
		&entry.Timestamp,
		&entry.URL,
		&entry.Method,
		&entry.GetParams,
		&entry.Data,
		&entry.Headers,
		&entry.ResponseHeaders,
		&entry.RawResponseBody,
		&entry.ContentType,
		&durationNs,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	entry.Duration = time.Duration(durationNs)
	return entry, nil
}

// Clear removes all entries from the history
func Clear() error {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return fmt.Errorf("history not initialized")
	}

	_, err := db.Exec("DELETE FROM history")
	if err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	return nil
}

// Count returns the total number of entries in the history
func Count() (int, error) {
	mu.RLock()
	defer mu.RUnlock()

	if !initialized {
		return 0, fmt.Errorf("history not initialized")
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM history").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entries: %w", err)
	}

	return count, nil
}
