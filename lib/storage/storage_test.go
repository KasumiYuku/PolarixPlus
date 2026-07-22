package storage

import (
	"path/filepath"
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
