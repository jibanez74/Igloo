package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	FirstName           string         `gorm:"size:30;not null" json:"firstName"`
	LastName            string         `gorm:"size:30;not null" json:"lastName"`
	Email               string         `gorm:"not null;uniqueIndex" json:"email"`
	Username            string         `gorm:"size:20;not null;uniqueIndex" json:"username"`
	Password            string         `gorm:"size:128;not null" json:"password"`
	Avatar              string         `gorm:"default:'/public/images/no_avatar.jpg'" json:"avatar"`
	IsAdmin             bool           `gorm:"default:false; not null"`
	FavoriteMusicians   []*Musician    `gorm:"many2many:user_musicians" json:"favoriteMusicians"`
	FavoriteMusicGenres []*MusicGenre  `gorm:"many2many:user_genres" json:"favoriteGenres"`
	Playlists           []*Playlist    `gorm:"constraint:OnDelete:CASCADE" json:"playlists"`
	TrackHistory        []TrackHistory `gorm:"constraint:OnDelete:CASCADE" json:"trackHistory"`
}

func (m *User) BeforeSave(tx *gorm.DB) (err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(m.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	m.Password = string(hashedPassword)

	return nil
}

func (m *User) PasswordMatches(plainText string) error {
	err := bcrypt.CompareHashAndPassword([]byte(m.Password), []byte(plainText))
	if err != nil {
		return err
	}

	return nil
}

func (m *User) BeforeDelete(tx *gorm.DB) (err error) {
	if err = tx.Model(m).Association("Musicians").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Genres").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Tracks").Clear(); err != nil {
		return err
	}

	return nil
}
