package task

import (
	"errors"
	"testing"
)

func TestTryRegisterFileMigrationRejectsActiveTask(t *testing.T) {
	tasksMu.Lock()
	previous := tasks
	tasks = make(map[string]Task)
	tasksMu.Unlock()
	t.Cleanup(func() {
		tasksMu.Lock()
		tasks = previous
		tasksMu.Unlock()
	})

	first := NewFileMigrationTask(42, "uuid-1", "a.bin", 18, 19, 10)
	if err := TryRegisterFileMigration(first); err != nil {
		t.Fatal(err)
	}

	second := NewFileMigrationTask(42, "uuid-1", "a.bin", 18, 19, 10)
	err := TryRegisterFileMigration(second)
	if !errors.Is(err, ErrMigrationAlreadyRunning) {
		t.Fatalf("expected active migration to be rejected, got %v", err)
	}

	other := NewFileMigrationTask(43, "uuid-2", "b.bin", 18, 19, 10)
	if err := TryRegisterFileMigration(other); err != nil {
		t.Fatal(err)
	}

	first.finishWith(TaskStatusFailed, nil)
	third := NewFileMigrationTask(42, "uuid-1", "a.bin", 18, 19, 10)
	if err := TryRegisterFileMigration(third); err != nil {
		t.Fatalf("expected a new migration after the previous one finished, got %v", err)
	}
}
