package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
)

var db *gorm.DB // not my proudest code

type Persistence struct{}

func (p *Persistence) Initialize(dsn string) {
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	db.AutoMigrate(&Email{})
}
