package task

import (
	"errors"
	"sync"
	"time"
)

// ErrMigrationAlreadyRunning is returned when a file already has a migration
// that has not finished.
var ErrMigrationAlreadyRunning = errors.New("file migration is already running")

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type Task interface {
	ID() string
	Run() error
	Progress() float64
	Status() TaskStatus
	Error() error
	Stop()
	FinishTime() time.Time
}

var (
	tasks   = make(map[string]Task)
	tasksMu sync.RWMutex
)

func RegisterTask(task Task) {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	tasks[task.ID()] = task
}

// TryRegisterFileMigration registers a migration unless this file already has
// one that is still pending or running.
func TryRegisterFileMigration(task *FileMigrationTask) error {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	for _, existing := range tasks {
		migration, ok := existing.(*FileMigrationTask)
		if !ok || migration.fileID != task.fileID || migration.ID() == task.ID() {
			continue
		}
		if !migration.isTerminal() {
			return ErrMigrationAlreadyRunning
		}
	}
	tasks[task.ID()] = task
	return nil
}

func GetTask(id string) (Task, bool) {
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	task, exists := tasks[id]
	return task, exists
}

func StopTask(id string) {
	tasksMu.RLock()
	task, exists := tasks[id]
	tasksMu.RUnlock()

	if exists {
		task.Stop()
	}
}

func GetAllTasks() []Task {
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	allTasks := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		allTasks = append(allTasks, task)
	}
	return allTasks
}

func CleanupTasks() {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	for id, task := range tasks {
		if task.Status() == TaskStatusCompleted || task.Status() == TaskStatusFailed {
			if time.Since(task.FinishTime()) > 24*time.Hour {
				delete(tasks, id)
			}
		}
	}
}

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				CleanupTasks()
			}
		}
	}()
}
