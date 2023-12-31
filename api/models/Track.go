// description: defines the track model and its hooks
package models

import (
	"gorm.io/gorm"
)

type Track struct {
	gorm.Model
	Title        string `gorm:"size:60;not null" json:"title"`
	Index        uint   `gorm:"not null; default:0" json:"index"`
	RemoteFile   string `gorm:"size:255;not null" json:"remoteFile"`
	LocalFile    string `gorm:"size:255" json:"localFile"`
	IsDownload   bool   `gorm:"not null; default:false" json:"isDownload"`
	Duration     uint   `gorm:"not null" json:"duration"`
	Container    string `gorm:"size:10;not null" json:"container"`
	Bitrate      uint   `gorm:"not null" json:"bitrate"`
	Channels     uint   `gorm:"not null" json:"channels"`
	Language     string `gorm:"size:10; not null; default:'unknown'" json:"language"`
	Size         uint   `gorm:"not null; default:0" json:"size"`
	MusicBrainID string `gorm:"size:255;not null; default:'unknown'" json:"musicBrainID"`
	Summary      string `gorm:"type:text" json:"summary"`
	Codec        string `gorm:"size:10;not null" json:"codec"`
	AlbumID      uint
	Album        Album
	Musicians    []*Musician     `gorm:"many2many:musician_tracks;" json:"musicians"`
	Genres       []*MusicGenre   `gorm:"many2many:track_genres;" json:"genres"`
	Moods        []*Mood         `gorm:"many2many:track_moods;" json:"moods"`
	Playlists    []*Playlist     `gorm:"many2many:playlist_tracks;" json:"playlists"`
	PlayHistory  []*TrackHistory `gorm:"many2many:track_history" json:"playHistory"`
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
