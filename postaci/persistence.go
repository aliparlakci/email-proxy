package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
)

var db *gorm.DB // TODO: no globals

type Persistence struct{}

type EmailFinder interface {
	FindEmail(emailId uint64) (*Email, error)
}

func (p *Persistence) FindEmail(emailId uint64) (*Email, error) {
	var email Email
	result := db.Find(&email, emailId)
	return &email, result.Error
}

func (p *Persistence) Initialize(dsn string) {
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	db.AutoMigrate(&Email{})
}

func (p *Persistence) InitializeTesting() {
	var err error
	db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	db.AutoMigrate(&Email{})
}
