package history

import "database/sql"

// initSchema creates the history table and indexes if they don't exist
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		url TEXT NOT NULL,
		method TEXT NOT NULL,
		get_params TEXT,
		data TEXT,
		headers TEXT,
		response_headers TEXT,
		raw_response_body BLOB,
		content_type TEXT,
		duration_ns INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_timestamp ON history(timestamp DESC);
	`

	_, err := db.Exec(schema)
	return err
}
