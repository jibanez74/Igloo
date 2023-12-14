// description: defines the repo models and funcs for a user
package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(u *models.User) error
	FindByID(id int) (*models.User, error)
	FindByEmailAndUsername(email string, username string) (*models.User, error)
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{
		db: db,
	}
}

func (r *userRepo) Create(u *models.User) error {
	return r.db.Create(u).Error
}

func (r *userRepo) FindByID(id int) (*models.User, error) {
	var user models.User

	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *userRepo) FindByEmailAndUsername(email string, username string) (*models.User, error) {
	var user models.User

	if err := r.db.Where("email = ? AND username = ?", email, username).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
