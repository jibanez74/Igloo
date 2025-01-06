package repository

import "igloo/cmd/database/models"

func (r *repo) GetSettings() (*models.GlobalSettings, error) {
	var settings models.GlobalSettings

	err := r.db.First(&settings).Error
	if err != nil {
		return nil, err
	}

	return &settings, nil
}
