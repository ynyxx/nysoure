package storage

import (
	"errors"
	"nysoure/server/model"
)

var (
	// ErrFileUnavailable is returned when the file is unavailable.
	// When this error is returned, it is required to delete the file info from the database.
	ErrFileUnavailable = errors.New("file unavailable")
	// ErrObjectNotFound is returned when Delete finds no live object for the key.
	ErrObjectNotFound = errors.New("storage object not found")
)

type IStorage interface {
	// Upload uploads a file to the storage and returns the storage key.
	Upload(filePath string, fileName string) (string, error)
	// Download return the download url of the file with the given storage key.
	Download(storageKey string, fileName string) (string, error)
	// Delete deletes the file with the given storage key.
	Delete(storageKey string) error
	// ToString returns the storage configuration as a string.
	ToString() string
	// FromString initializes the storage configuration from a string.
	FromString(config string) error
	// Type returns the type of the storage.
	Type() string
}

func NewStorage(s model.Storage) IStorage {
	switch s.Type {
	case "s3":
		r := S3Storage{}
		err := r.FromString(s.Config)
		if err != nil {
			return nil
		}
		return &r

	case "local":
		r := LocalStorage{}
		err := r.FromString(s.Config)
		if err != nil {
			return nil
		}
		return &r

	case "ftp":
		r := FTPStorage{}
		err := r.FromString(s.Config)
		if err != nil {
			return nil
		}
		return &r

	case "gdrive":
		r := GoogleDriveStorage{}
		err := r.FromString(s.Config)
		if err != nil {
			return nil
		}
		return &r
	}
	return nil
}
