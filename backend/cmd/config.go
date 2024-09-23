package main

import (
	"log"

	"gorm.io/gorm"
)

type config struct {
	DB       *gorm.DB
	InfoLog  *log.Logger
	ErrorLog *log.Logger
}
