package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	ScopeGlobal  = "global"
	ScopePlugin  = "plugin"
	ScopeCommand = "command"
	ScopeUser    = "user"
	ScopeGroup   = "group"
)

var (
	db   *sql.DB
	dbMu sync.RWMutex
)

// Store is a key-value namespace bound to a specific data scope.
type Store struct {
	scope     string
	namespace string
}

func Open(path string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		return errors.New("storage is already open")
	}

	opened, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	if err = opened.Ping(); err != nil {
		opened.Close()
		return fmt.Errorf("connect to sqlite database: %w", err)
	}

	if _, err = opened.Exec(`
		CREATE TABLE IF NOT EXISTS kv_data (
			scope TEXT NOT NULL,
			namespace TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (scope, namespace, key)
		)
	`); err != nil {
		opened.Close()
		return fmt.Errorf("initialize sqlite database: %w", err)
	}

	db = opened
	return nil
}

func Close() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

func Global() *Store {
	return &Store{scope: ScopeGlobal, namespace: ScopeGlobal}
}

func Plugin(pluginID string) *Store {
	return &Store{scope: ScopePlugin, namespace: pluginID}
}

func Command(pluginID, commandID string) *Store {
	return &Store{scope: ScopeCommand, namespace: pluginID + ":" + commandID}
}

func User(userID string) *Store {
	return &Store{scope: ScopeUser, namespace: userID}
}

func Group(groupID string) *Store {
	return &Store{scope: ScopeGroup, namespace: groupID}
}

func (store *Store) Set(key string, value any) error {
	if err := store.validate(key); err != nil {
		return err
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal storage value: %w", err)
	}
	database, err := currentDB()
	if err != nil {
		return err
	}

	_, err = database.Exec(`
		INSERT INTO kv_data (scope, namespace, key, value, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(scope, namespace, key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, store.scope, store.namespace, key, encoded)
	if err != nil {
		return fmt.Errorf("set storage value: %w", err)
	}
	return nil
}

func (store *Store) Get(key string, target any) (bool, error) {
	if err := store.validate(key); err != nil {
		return false, err
	}
	if target == nil {
		return false, errors.New("storage target cannot be nil")
	}

	database, err := currentDB()
	if err != nil {
		return false, err
	}
	var encoded []byte
	err = database.QueryRow(
		"SELECT value FROM kv_data WHERE scope = ? AND namespace = ? AND key = ?",
		store.scope, store.namespace, key,
	).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get storage value: %w", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		return false, fmt.Errorf("unmarshal storage value: %w", err)
	}
	return true, nil
}

func (store *Store) Has(key string) (bool, error) {
	if err := store.validate(key); err != nil {
		return false, err
	}
	database, err := currentDB()
	if err != nil {
		return false, err
	}

	var exists int
	err = database.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM kv_data WHERE scope = ? AND namespace = ? AND key = ?)",
		store.scope, store.namespace, key,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check storage value: %w", err)
	}
	return exists == 1, nil
}

func (store *Store) Delete(key string) error {
	if err := store.validate(key); err != nil {
		return err
	}
	database, err := currentDB()
	if err != nil {
		return err
	}
	if _, err = database.Exec(
		"DELETE FROM kv_data WHERE scope = ? AND namespace = ? AND key = ?",
		store.scope, store.namespace, key,
	); err != nil {
		return fmt.Errorf("delete storage value: %w", err)
	}
	return nil
}

func (store *Store) Clear() error {
	if store == nil || store.scope == "" || store.namespace == "" {
		return errors.New("invalid storage namespace")
	}
	database, err := currentDB()
	if err != nil {
		return err
	}
	if _, err = database.Exec(
		"DELETE FROM kv_data WHERE scope = ? AND namespace = ?",
		store.scope, store.namespace,
	); err != nil {
		return fmt.Errorf("clear storage namespace: %w", err)
	}
	return nil
}

func (store *Store) validate(key string) error {
	if store == nil || store.scope == "" || store.namespace == "" {
		return errors.New("invalid storage namespace")
	}
	if key == "" {
		return errors.New("storage key cannot be empty")
	}
	return nil
}

func currentDB() (*sql.DB, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()
	if db == nil {
		return nil, errors.New("storage is not open")
	}
	return db, nil
}
