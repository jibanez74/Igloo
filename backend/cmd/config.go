package main

import (
	"log"

	"github.com/alexedwards/scs/v2"
	"gorm.io/gorm"
)

type config struct {
	DB       *gorm.DB
	InfoLog  *log.Logger
	ErrorLog *log.Logger
	Session  *scs.SessionManager
}
