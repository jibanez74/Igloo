package models

import (
	"errors"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name     string `gorm:"not null" json:"name"`
	Username string `gorm:"not null;uniqueIndex" json:"username"`
	Email    string `gorm:"not null;uniqueIndex" json:"email"`
	Password string `gorm:"not null" json:"password"`
	Thumb    string `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	Active   bool   `gorm:"not null;default:false" json:"active"`
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.Name == "" {
		return tx.AddError(errors.New("name is required"))
	}

	if u.Username == "" {
		return tx.AddError(errors.New("username is required"))
	}

	if u.Email == "" {
		return tx.AddError(errors.New("email is required"))
	}

	if len(u.Password) < 9 || len(u.Password) > 128 {
		return tx.AddError(errors.New("password must be between 9 and 128 characters"))
	}

	return nil
}
