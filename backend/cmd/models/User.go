package models

import (
	"errors"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name      string `gorm:"not null;size:60"`
	Username  string `gorm:"not null;uniqueIndex"`
	Email     string `gorm:"not null;uniqueIndex"`
	Password  string `gorm:"not null;size:128"`
	IsAdmin   bool   `gorm:"default:false"`
	IsEnabled bool   `gorm:"default:false"`
	Thumb     string `gorm:"not null"`
}

func (u *User) ComparePassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		log.Print(err)
		return false
	}

	return true
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func (u *User) SetPassword(password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	u.Password = hashedPassword

	return nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Name == "" || u.Email == "" || u.Password == "" {
		return errors.New("name, email and password are required")
	}

	hashedPassword, err := hashPassword(u.Password)
	if err != nil {
		return err
	}

	u.Password = hashedPassword

	return nil
}
