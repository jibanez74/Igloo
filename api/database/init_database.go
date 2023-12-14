package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDatabase() (*gorm.DB, error) {
	dsn := "host=localhost user=postgres password=postgres dbname=igloo port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	RunMigrations(db)

	return db, nil
}
