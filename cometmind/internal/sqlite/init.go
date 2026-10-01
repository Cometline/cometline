package sqlite

import "database/sql"

// modernc.org/sqlite races in sqlite3_initialize when the first connections
// are opened concurrently. That init is not synchronized until it finishes,
// and the race detector flags the mutex-method setup. Package init runs on
// one goroutine, so later Open calls take the already-initialized path.
func init() {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	pingErr := conn.Ping()
	closeErr := conn.Close()
	if pingErr != nil {
		panic(pingErr)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}
