package dao

import (
	"nysoure/server/model"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

var (
	ready = false
)

func InitDB() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dsn := "host=" + host + " user=" + user + " password=" + password + " dbname=" + dbName + " port=" + port + " sslmode=disable"
	var err error
	// wait for postgres to be ready
	retrys := 5
	for {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			ready = true
			break
		}
		retrys--
		if retrys < 0 {
			panic("failed to connect database: " + err.Error())
		}
		time.Sleep(1 * time.Second)
	}

	_ = db.AutoMigrate(
		&model.User{},
		&model.Resource{},
		&model.Image{},
		&model.Tag{},
		&model.Storage{},
		&model.File{},
		&model.UploadingFile{},
		&model.Statistic{},
		&model.Comment{},
		&model.Activity{},
		&model.Collection{},
		&model.CollectionResource{},
		&model.Character{},
		&model.Relation{},
		&model.TagTemplate{},
	)
}

func GetDB() *gorm.DB {
	return db
}

func IsReady() bool {
	return ready
}

func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
