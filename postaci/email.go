package main

import (
	"gorm.io/gorm"
	"time"
)

type Email struct {
	gorm.Model
	From     string
	To       string
	Filename string
	SentDate time.Time
	Content  []byte
}
