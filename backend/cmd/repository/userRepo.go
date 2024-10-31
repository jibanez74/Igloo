package repository

import (
	"igloo/cmd/database/models"
	"net/http"
)

func (r *repo) GetAuthUser(user *models.User) (int, error) {
	err := r.db.Where("email = ? AND username = ? AND is_active = ?", user.Email, user.Username, user.IsActive).First(&user).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusOK, nil
}

func (r *repo) GetUserByID(user *models.User) (int, error) {
	err := r.db.First(&user, user.ID).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusOK, nil
}

func (r *repo) CreateUser(user *models.User) (int, error) {
	err := r.db.Create(&user).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusCreated, nil
}
