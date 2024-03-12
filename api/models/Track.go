package models

import (
	"gorm.io/gorm"
)

type Track struct {
	gorm.Model
	Title       string `gorm:"size:100;not null;index" json:"title"`
	Index       uint   `gorm:"default:0" json:"index"`
	File        string `gorm:"not null;index" json:"file"`
	Duration    uint   `gorm:"not null" json:"duration"`
	Year        uint   `json:"year"`
	Container   string `gorm:"size:10;not null" json:"container"`
	Bitrate     uint   `gorm:"not null" json:"bitrate"`
	Channels    uint   `gorm:"not null" json:"channels"`
	Language    string `gorm:"size:10; not null; default:'unknown'" json:"language"`
	Size        uint   `gorm:"not null" json:"size"`
	Codec       string `gorm:"size:10;not null" json:"codec"`
	AlbumID     uint
	Album       Album
	Musicians   []*Musician     `gorm:"many2many:musician_tracks;" json:"musicians"`
	Genres      []*MusicGenre   `gorm:"many2many:track_genres;" json:"genres"`
	Moods       []*MusicMood    `gorm:"many2many:track_moods" json:"moods"`
	Playlists   []*Playlist     `gorm:"many2many:playlist_tracks;" json:"playlists"`
	PlayHistory []*TrackHistory `gorm:"many2many:track_history" json:"playHistory"`
}

func (m *Track) BeforeDelete(tx *gorm.DB) (err error) {
	if err := tx.Model(&Album{}).Where("id = ?", m.AlbumID).Association("Tracks").Delete(m); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Musicians").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Moods").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Genres").Clear(); err != nil {
		return err
	}

	return nil
}

func (m *Track) AfterDelete(tx *gorm.DB) (err error) {
	var album Album

	if err := tx.Model(&Album{}).Where("id = ?", m.AlbumID).First(&album).Error; err != nil {
		return err
	}

	if album.NumberOfTracks > 0 {
		album.NumberOfTracks--

		if err := tx.Save(&album).Error; err != nil {
			return err
		}
	}

	return nil
}

// func getContentType(container string) string {
// 	switch container {
// 	case "mp3":
// 		return "audio/mpeg"
// 	case "m4a":
// 		return "audio/mp4"
// 	case "flac":
// 		return "audio/flac"
// 	default:
// 		return "application/octet-stream" // Default to binary data if the container is unknown
// 	}
// }
