package models

import (
	"gorm.io/gorm"
)

type Track struct {
	gorm.Model
	Title        string `gorm:"not null" json:"title"`
	Index        uint   `json:"index"`
	FilePath     string `gorm:"not null" json:"filePath"`
	Duration     uint   `gorm:"not null" json:"duration"`
	Container    string `gorm:"size:10;not null" json:"container"`
	Language     string `gorm:"default:'unknown'" json:"language"`
	Size         uint   `gorm:"not null; default:0" json:"size"`
	MusicBrainID string `gorm:"default:'unknown'" json:"musicBrainID"`
	Summary      string `gorm:"type:text" json:"summary"`
	AlbumID      uint
	Album        Album
	Musicians    []*Musician     `gorm:"many2many:musician_tracks;" json:"musicians"`
	Genres       []*Genre        `gorm:"many2many:track_genres;" json:"genres"`
	Moods        []*Mood         `gorm:"many2many:track_moods;" json:"moods"`
	Playlists    []*Playlist     `gorm:"many2many:playlist_tracks;" json:"playlists"`
	PlayHistory  []*TrackHistory `gorm:"many2many:track_history" json:"playHistory"`
}
