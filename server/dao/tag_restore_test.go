package dao

import (
	"errors"
	"os"
	"testing"

	"nysoure/server/model"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505", Message: "duplicate key"}) {
		t.Fatal("pg 23505 should be a unique violation")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("foreign key violation is not a unique violation")
	}
	if !isUniqueViolation(gorm.ErrDuplicatedKey) {
		t.Fatal("gorm duplicated key should be a unique violation")
	}
	wrapped := errors.Join(errors.New("create tag"), &pgconn.PgError{Code: "23505"})
	if !isUniqueViolation(wrapped) {
		t.Fatal("wrapped pg 23505 should be a unique violation")
	}
	if isUniqueViolation(errors.New("connection reset")) {
		t.Fatal("unrelated error should not be a unique violation")
	}
	if isUniqueViolation(nil) {
		t.Fatal("nil error should not be a unique violation")
	}
}

func TestCreateTagRestoresSoftDeletedName(t *testing.T) {
	dsn := os.Getenv("TAG_TEST_DSN")
	if dsn == "" {
		t.Skip("TAG_TEST_DSN not set")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&model.Tag{}); err != nil {
		t.Fatal(err)
	}
	prev := db
	db = gdb
	t.Cleanup(func() {
		_ = gdb.Exec("DELETE FROM tags WHERE name = ?", "restore-tag-test").Error
		db = prev
		sqlDB, err := gdb.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	created, err := CreateTagWithType("restore-tag-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Delete(&model.Tag{}, created.ID).Error; err != nil {
		t.Fatal(err)
	}

	restored, err := CreateTagWithType("restore-tag-test", "画师")
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != created.ID {
		t.Fatalf("restored id %d, want %d", restored.ID, created.ID)
	}
	if restored.Type != "画师" {
		t.Fatalf("type %q, want 画师", restored.Type)
	}
	if restored.DeletedAt.Valid {
		t.Fatal("restored tag is still deleted")
	}

	again, err := CreateTagWithType("restore-tag-test", "剧本家")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatalf("existing id %d, want %d", again.ID, created.ID)
	}
	if again.Type != "画师" {
		t.Fatalf("type overwritten to %q", again.Type)
	}
}
