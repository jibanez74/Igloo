package repository

import (
	"igloo/cmd/models"
)

func (r *Repo) FindOrCreateArtist(artist *models.Artist) error {
	return r.db.Where("name = ?", artist.Name).FirstOrCreate(artist).Error
}
