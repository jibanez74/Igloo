package models

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name           string   `gorm:"not null" json:"name"`
	Username       string   `gorm:"not null;uniqueIndex" json:"username"`
	Email          string   `gorm:"not null;uniqueIndex" json:"email"`
	Password       string   `gorm:"not null" json:"password"`
	Thumb          string   `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	Active         bool     `gorm:"not null;default:false" json:"active"`
	IsAdmin        bool     `gorm:"default:false" json:"isAdmin"`
	FavoriteMovies []*Movie `gorm:"many2many:user_movies" json:"favoriteMovies"`
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return tx.AddError(err)
	}
	u.Password = string(hashedPassword)

	return nil
}

func (u *User) PasswordMatches(plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainText))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			// invalid password
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}
