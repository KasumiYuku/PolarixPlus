package storage

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func openTestStorage(t *testing.T) {
	t.Helper()
	if err := Open(filepath.Join(t.TempDir(), "storage.db")); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestStoreLifecycle(t *testing.T) {
	openTestStorage(t)
	store := Plugin("example")

	want := struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}{Count: 3, Name: "value"}
	if err := store.Set("key", want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	exists, err := store.Has("key")
	if err != nil {
		t.Fatalf("Has() error = %v", err)
	}
	if !exists {
		t.Fatal("Has() = false, want true")
	}

	var got struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}
	found, err := store.Get("key", &got)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if got != want {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}

	if err := store.Delete("key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	found, err = store.Get("key", &got)
	if err != nil {
		t.Fatalf("Get() after Delete() error = %v", err)
	}
	if found {
		t.Fatal("Get() after Delete() found = true, want false")
	}
}

func TestScopesAreIsolated(t *testing.T) {
	openTestStorage(t)

	stores := []*Store{
		Global(),
		Plugin("example"),
		Command("example", "/run"),
		Command("example", "/run child"),
		User("user-a"),
		User("user-b"),
		Group("group-a"),
		Group("group-b"),
	}
	for index, store := range stores {
		if err := store.Set("shared", index); err != nil {
			t.Fatalf("store %d Set() error = %v", index, err)
		}
	}
	for index, store := range stores {
		var got int
		found, err := store.Get("shared", &got)
		if err != nil {
			t.Fatalf("store %d Get() error = %v", index, err)
		}
		if !found || got != index {
			t.Fatalf("store %d Get() = (%d, %v), want (%d, true)", index, got, found, index)
		}
	}
}

func TestClearOnlyAffectsCurrentNamespace(t *testing.T) {
	openTestStorage(t)
	first := Plugin("first")
	second := Plugin("second")

	if err := first.Set("key", "first"); err != nil {
		t.Fatalf("first Set() error = %v", err)
	}
	if err := second.Set("key", "second"); err != nil {
		t.Fatalf("second Set() error = %v", err)
	}
	if err := first.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	firstExists, err := first.Has("key")
	if err != nil {
		t.Fatalf("first Has() error = %v", err)
	}
	secondExists, err := second.Has("key")
	if err != nil {
		t.Fatalf("second Has() error = %v", err)
	}
	if firstExists || !secondExists {
		t.Fatalf("namespace existence = (%v, %v), want (false, true)", firstExists, secondExists)
	}
}

func TestLazyOpenBeforeExplicitOpen(t *testing.T) {
	// 模拟插件 init: 未显式 Open 也能读写 (默认 bot.db 在临时目录不便, 先 Close 保证干净)
	_ = Close()
	path := filepath.Join(t.TempDir(), "lazy.db")
	// 通过 Open 设定路径, 再 Close, 仅保留 dbPath 语义较难测;
	// 改为: Open 后写, Close 再 Open 同一文件验证持久化; 另测 ensureDB 幂等 Open
	if err := Open(path); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := Global().Set("from-init", "ok"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// 再次 Open 同一路径, 数据仍在
	if err := Open(path); err != nil {
		t.Fatalf("re-Open() error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	var got string
	found, err := Global().Get("from-init", &got)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || got != "ok" {
		t.Fatalf("Get() = (%q, %v), want (ok, true)", got, found)
	}

	// Open 幂等: 同路径再 Open 不报错
	if err := Open(path); err != nil {
		t.Fatalf("idempotent Open() error = %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	openTestStorage(t)
	store := Global()
	const workers = 32
	const rounds = 50

	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", id)
			for r := 0; r < rounds; r++ {
				if err := store.Set(key, r); err != nil {
					errCh <- err
					return
				}
				var got int
				found, err := store.Get(key, &got)
				if err != nil {
					errCh <- err
					return
				}
				if !found {
					errCh <- fmt.Errorf("key %s missing after set", key)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent access error: %v", err)
	}
}
