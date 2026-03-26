package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// resetHistory closes and resets the history state for testing
func resetHistory() {
	Close()
	db = nil
	initialized = false
}

// TestInit tests the Init function
func TestInit(t *testing.T) {
	defer resetHistory()

	err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !initialized {
		t.Error("Expected initialized to be true")
	}

	if db == nil {
		t.Error("Expected db to be initialized")
	}
}

// TestInitTwice tests that calling Init twice returns an error
func TestInitTwice(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("First Init failed: %v", err)
	}

	err := Init(":memory:")
	if err == nil {
		t.Error("Expected error when calling Init twice")
	}
}

// TestInitWithFile tests Init with a real file
func TestInitWithFile(t *testing.T) {
	defer resetHistory()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	err := Init(dbPath)
	if err != nil {
		t.Fatalf("Init with file failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

// TestClose tests the Close function
func TestClose(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if initialized {
		t.Error("Expected initialized to be false after Close")
	}

	if db != nil {
		t.Error("Expected db to be nil after Close")
	}
}

// TestCloseWithoutInit tests Close when not initialized
func TestCloseWithoutInit(t *testing.T) {
	defer resetHistory()

	err := Close()
	if err != nil {
		t.Errorf("Close without init should not error: %v", err)
	}
}

// TestAddEntry tests adding a single entry
func TestAddEntry(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	entry := &Entry{
		Timestamp:       time.Now(),
		URL:             "https://example.com",
		Method:          "GET",
		GetParams:       "foo=bar",
		Data:            "test data",
		Headers:         "Content-Type: application/json",
		ResponseHeaders: "Content-Length: 100",
		RawResponseBody: []byte("response body"),
		ContentType:     "application/json",
		Duration:        100 * time.Millisecond,
	}

	err := AddEntry(entry)
	if err != nil {
		t.Errorf("AddEntry failed: %v", err)
	}

	if entry.ID == 0 {
		t.Error("Expected entry ID to be set after AddEntry")
	}
}

// TestAddEntryWithoutInit tests AddEntry when not initialized
func TestAddEntryWithoutInit(t *testing.T) {
	defer resetHistory()

	entry := &Entry{
		Timestamp: time.Now(),
		URL:       "https://example.com",
		Method:    "GET",
	}

	err := AddEntry(entry)
	if err == nil {
		t.Error("Expected error when calling AddEntry without Init")
	}
}

// TestGetHistory tests retrieving all history entries
func TestGetHistory(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// Add multiple entries
	entries := []*Entry{
		{
			Timestamp: time.Now().Add(-2 * time.Hour),
			URL:       "https://example.com/1",
			Method:    "GET",
		},
		{
			Timestamp: time.Now().Add(-1 * time.Hour),
			URL:       "https://example.com/2",
			Method:    "POST",
		},
		{
			Timestamp: time.Now(),
			URL:       "https://example.com/3",
			Method:    "PUT",
		},
	}

	for _, entry := range entries {
		if err := AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Get all entries
	retrieved, err := GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(retrieved))
	}

	// Verify newest first ordering
	if retrieved[0].URL != "https://example.com/3" {
		t.Errorf("Expected newest entry first, got %s", retrieved[0].URL)
	}
	if retrieved[2].URL != "https://example.com/1" {
		t.Errorf("Expected oldest entry last, got %s", retrieved[2].URL)
	}
}

// TestGetHistoryEmpty tests GetHistory with no entries
func TestGetHistoryEmpty(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	entries, err := GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

// TestGetHistoryWithoutInit tests GetHistory when not initialized
func TestGetHistoryWithoutInit(t *testing.T) {
	defer resetHistory()

	_, err := GetHistory()
	if err == nil {
		t.Error("Expected error when calling GetHistory without Init")
	}
}

// TestGetEntryByID tests retrieving a specific entry by ID
func TestGetEntryByID(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	entry := &Entry{
		Timestamp:       time.Now(),
		URL:             "https://example.com",
		Method:          "GET",
		GetParams:       "foo=bar",
		Data:            "test data",
		Headers:         "Content-Type: application/json",
		ResponseHeaders: "Content-Length: 100",
		RawResponseBody: []byte("response body"),
		ContentType:     "application/json",
		Duration:        100 * time.Millisecond,
	}

	if err := AddEntry(entry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Retrieve by ID
	retrieved, err := GetEntryByID(entry.ID)
	if err != nil {
		t.Fatalf("GetEntryByID failed: %v", err)
	}

	// Verify all fields
	if retrieved.ID != entry.ID {
		t.Errorf("Expected ID %d, got %d", entry.ID, retrieved.ID)
	}
	if retrieved.URL != entry.URL {
		t.Errorf("Expected URL %s, got %s", entry.URL, retrieved.URL)
	}
	if retrieved.Method != entry.Method {
		t.Errorf("Expected Method %s, got %s", entry.Method, retrieved.Method)
	}
	if retrieved.GetParams != entry.GetParams {
		t.Errorf("Expected GetParams %s, got %s", entry.GetParams, retrieved.GetParams)
	}
	if retrieved.Data != entry.Data {
		t.Errorf("Expected Data %s, got %s", entry.Data, retrieved.Data)
	}
	if retrieved.Headers != entry.Headers {
		t.Errorf("Expected Headers %s, got %s", entry.Headers, retrieved.Headers)
	}
	if retrieved.ResponseHeaders != entry.ResponseHeaders {
		t.Errorf("Expected ResponseHeaders %s, got %s", entry.ResponseHeaders, retrieved.ResponseHeaders)
	}
	if string(retrieved.RawResponseBody) != string(entry.RawResponseBody) {
		t.Errorf("Expected RawResponseBody %s, got %s", entry.RawResponseBody, retrieved.RawResponseBody)
	}
	if retrieved.ContentType != entry.ContentType {
		t.Errorf("Expected ContentType %s, got %s", entry.ContentType, retrieved.ContentType)
	}
	if retrieved.Duration != entry.Duration {
		t.Errorf("Expected Duration %v, got %v", entry.Duration, retrieved.Duration)
	}
}

// TestGetEntryByIDNotFound tests GetEntryByID with a non-existent ID
func TestGetEntryByIDNotFound(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	_, err := GetEntryByID(999)
	if err == nil {
		t.Error("Expected error when getting non-existent entry")
	}
}

// TestGetEntryByIDWithoutInit tests GetEntryByID when not initialized
func TestGetEntryByIDWithoutInit(t *testing.T) {
	defer resetHistory()

	_, err := GetEntryByID(1)
	if err == nil {
		t.Error("Expected error when calling GetEntryByID without Init")
	}
}

// TestClear tests clearing all history entries
func TestClear(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// Add some entries
	for i := 0; i < 5; i++ {
		entry := &Entry{
			Timestamp: time.Now(),
			URL:       "https://example.com",
			Method:    "GET",
		}
		if err := AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Clear history
	err := Clear()
	if err != nil {
		t.Errorf("Clear failed: %v", err)
	}

	// Verify history is empty
	entries, err := GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after Clear, got %d", len(entries))
	}
}

// TestClearWithoutInit tests Clear when not initialized
func TestClearWithoutInit(t *testing.T) {
	defer resetHistory()

	err := Clear()
	if err == nil {
		t.Error("Expected error when calling Clear without Init")
	}
}

// TestCount tests counting history entries
func TestCount(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// Initially should be 0
	count, err := Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Add entries
	for i := 0; i < 10; i++ {
		entry := &Entry{
			Timestamp: time.Now(),
			URL:       "https://example.com",
			Method:    "GET",
		}
		if err := AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Should be 10
	count, err = Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}

	// Clear and count again
	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	count, err = Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 after Clear, got %d", count)
	}
}

// TestCountWithoutInit tests Count when not initialized
func TestCountWithoutInit(t *testing.T) {
	defer resetHistory()

	_, err := Count()
	if err == nil {
		t.Error("Expected error when calling Count without Init")
	}
}

// TestSpecialCharacters tests handling of special characters
func TestSpecialCharacters(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	entry := &Entry{
		Timestamp: time.Now(),
		URL:       "https://example.com/test?name=John's \"Test\"&emoji=🚀",
		Method:    "POST",
		GetParams: "name=John's \"Test\"&emoji=🚀",
		Data:      "{\"message\": \"Hello\\nWorld\", \"emoji\": \"🎉\"}",
		Headers:   "Content-Type: application/json\r\nAuthorization: Bearer token123",
	}

	if err := AddEntry(entry); err != nil {
		t.Fatalf("AddEntry with special characters failed: %v", err)
	}

	retrieved, err := GetEntryByID(entry.ID)
	if err != nil {
		t.Fatalf("GetEntryByID failed: %v", err)
	}

	if retrieved.URL != entry.URL {
		t.Errorf("Special characters not preserved in URL")
	}
	if retrieved.GetParams != entry.GetParams {
		t.Errorf("Special characters not preserved in GetParams")
	}
	if retrieved.Data != entry.Data {
		t.Errorf("Special characters not preserved in Data")
	}
}

// TestLargeResponseBody tests handling of large response bodies
func TestLargeResponseBody(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// Create a 1MB response body
	largeBody := make([]byte, 1024*1024)
	for i := range largeBody {
		largeBody[i] = byte(i % 256)
	}

	entry := &Entry{
		Timestamp:       time.Now(),
		URL:             "https://example.com",
		Method:          "GET",
		RawResponseBody: largeBody,
	}

	if err := AddEntry(entry); err != nil {
		t.Fatalf("AddEntry with large body failed: %v", err)
	}

	retrieved, err := GetEntryByID(entry.ID)
	if err != nil {
		t.Fatalf("GetEntryByID failed: %v", err)
	}

	if len(retrieved.RawResponseBody) != len(largeBody) {
		t.Errorf("Expected body length %d, got %d", len(largeBody), len(retrieved.RawResponseBody))
	}

	// Verify a few bytes
	for i := 0; i < 1000; i++ {
		if retrieved.RawResponseBody[i] != largeBody[i] {
			t.Errorf("Body content mismatch at byte %d", i)
			break
		}
	}
}

// TestDurationPrecision tests that duration is preserved correctly
func TestDurationPrecision(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	testDurations := []time.Duration{
		1 * time.Nanosecond,
		100 * time.Microsecond,
		50 * time.Millisecond,
		2 * time.Second,
		5 * time.Minute,
	}

	for _, duration := range testDurations {
		entry := &Entry{
			Timestamp: time.Now(),
			URL:       "https://example.com",
			Method:    "GET",
			Duration:  duration,
		}

		if err := AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}

		retrieved, err := GetEntryByID(entry.ID)
		if err != nil {
			t.Fatalf("GetEntryByID failed: %v", err)
		}

		if retrieved.Duration != duration {
			t.Errorf("Expected duration %v, got %v", duration, retrieved.Duration)
		}
	}
}

// TestMultipleEntriesOrdering tests that entries are ordered correctly
func TestMultipleEntriesOrdering(t *testing.T) {
	defer resetHistory()

	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// Add entries with specific timestamps
	baseTime := time.Now()
	urls := []string{
		"https://example.com/first",
		"https://example.com/second",
		"https://example.com/third",
		"https://example.com/fourth",
		"https://example.com/fifth",
	}

	for i, url := range urls {
		entry := &Entry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			URL:       url,
			Method:    "GET",
		}
		if err := AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Get all entries
	entries, err := GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	// Verify they're in reverse order (newest first)
	for i, entry := range entries {
		expectedURL := urls[len(urls)-1-i]
		if entry.URL != expectedURL {
			t.Errorf("Entry %d: expected URL %s, got %s", i, expectedURL, entry.URL)
		}
	}
}
