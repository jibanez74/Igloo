package main

import (
	"log"
	"os/exec"

	"github.com/alexedwards/scs/v2"
	"gorm.io/gorm"
)

type transcodeJob struct {
	ID      string
	UserID  uint
	Process *exec.Cmd
}

type config struct {
	DB            *gorm.DB
	InfoLog       *log.Logger
	ErrorLog      *log.Logger
	Session       *scs.SessionManager
	TranscodeJobs []transcodeJob
}
