package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrDataDirInUse means another live Server already owns this installation.
var ErrDataDirInUse = errors.New("Folio data directory is already in use by another server; stop it or choose a different data directory")

// processLock reserves a connection holding an EXCLUSIVE transaction in a
// separate rollback-journal SQLite file. The metadata database remains free to
// commit normally. SQLite implements the OS-level locking on each supported
// platform, and a process crash releases the lock without stale-file removal.
//
// See https://www.sqlite.org/lang_transaction.html#deferred_immediate_and_exclusive_transactions
// and https://www.sqlite.org/lockingv3.html. Never delete or replace the lock
// database while Folio is running, or put the data directory on a filesystem
// without reliable SQLite file locks.
type processLock struct {
	db   *sql.DB
	conn *sql.Conn
}

func acquireProcessLock(dir string) (lock *processLock, err error) {
	name := filepath.Join(dir, ".folio-process-lock.db")
	if info, e := os.Lstat(name); e == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("Folio process lock must be a regular file")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	db, e := sql.Open("sqlite", name)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	lock = &processLock{db: db}
	ok := false
	held := lock
	defer func() {
		if !ok {
			held.Close()
			lock = nil
		}
		if err != nil {
			var sqliteErr *sqlite.Error
			if errors.As(err, &sqliteErr) {
				code := sqliteErr.Code() & 0xff
				if code == sqlite3.SQLITE_BUSY || code == sqlite3.SQLITE_LOCKED {
					err = ErrDataDirInUse
					return
				}
			}
			err = fmt.Errorf("cannot acquire Folio data directory lock: %w", err)
		}
	}()
	// Conn is reserved for the Server lifetime. Use bounded statement contexts,
	// not BeginTx with a timeout: a BeginTx context ending would release its lock.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock.conn, err = db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(name, 0600); err != nil {
		return nil, err
	}
	if _, err = lock.conn.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		return nil, err
	}
	var mode string
	if err = lock.conn.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&mode); err != nil {
		return nil, err
	}
	if !strings.EqualFold(mode, "delete") {
		return nil, errors.New("process lock requires SQLite rollback-journal mode")
	}
	if _, err = lock.conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		return nil, err
	}
	ok = true
	return lock, nil
}
func (l *processLock) Close() error {
	if l == nil {
		return nil
	}
	var errs []error
	if l.conn != nil {
		_, e := l.conn.ExecContext(context.Background(), "ROLLBACK")
		errs = append(errs, e, l.conn.Close())
		l.conn = nil
	}
	if l.db != nil {
		errs = append(errs, l.db.Close())
		l.db = nil
	}
	return errors.Join(errs...)
}
