package database

import (
  "fmt"
  "igloo/cmd/internal/database/models"
  "time"

  "gorm.io/gorm"
)

type Migration struct {
  ID          uint           `gorm:"primarykey"`
  Version     uint           `gorm:"uniqueIndex;not null"`
  Name        string         `gorm:"size:255;not null"`
  Description string         `gorm:"type:text"`
  AppliedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP"`
  DeletedAt   gorm.DeletedAt `gorm:"index"`
}

type MigrationStep struct {
  Version     uint
  Name        string
  Description string
  Up          func(*gorm.DB) error
  Down        func(*gorm.DB) error // Added Down function for proper rollback
}

var migrations = []MigrationStep{
  {
    Version:     1,
    Name:        "initial_schema",
    Description: "Create initial database schema",
    Up: func(db *gorm.DB) error {
      return db.AutoMigrate(
        &models.GlobalSettings{},
        &models.Movie{},
        &models.Artist{},
        &models.Cast{},
        &models.Crew{},
        &models.Genre{},
        &models.Studio{},
        &models.MovieExtra{},
        &models.Chapter{},
        &models.Subtitles{},
        &models.AudioStream{},
        &models.VideoStream{},
        &models.User{},
      )
    },
    Down: func(db *gorm.DB) error {
      // In a real application, you might want to drop tables in reverse order
      // For the initial migration, this might not be needed
      return nil
    },
  },
  // Add new migrations here
}

func (Migration) TableName() string {
  return "schema_migrations"
}

func runMigrations(db *gorm.DB) error {
  err := db.AutoMigrate(&Migration{})
  if err != nil {
    return fmt.Errorf("failed to create migrations table: %w", err)
  }

  var lastMigration Migration

  result := db.Order("version desc").First(&lastMigration)
  lastVersion := uint(0)
  if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
    return fmt.Errorf("failed to get last migration: %w", result.Error)
  }

  if result.RowsAffected > 0 {
    lastVersion = lastMigration.Version
  }

  return db.Transaction(func(tx *gorm.DB) error {
    for _, migration := range migrations {
      if migration.Version <= lastVersion {
        continue
      }

      err = migration.Up(tx)
      if err != nil {
        return fmt.Errorf("failed to apply migration %d (%s): %w",
          migration.Version, migration.Name, err)
      }

      record := Migration{
        Version:     migration.Version,
        Name:        migration.Name,
        Description: migration.Description,
        AppliedAt:   time.Now().UTC(),
      }

      err = tx.Create(&record).Error
      if err != nil {
        return fmt.Errorf("failed to record migration %d: %w",
          migration.Version, err)
      }

      fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Name)
    }

    return nil
  })
}

func GetMigrationStatus(db *gorm.DB) ([]map[string]interface{}, error) {
  var appliedMigrations []Migration
  if err := db.Order("version asc").Find(&appliedMigrations).Error; err != nil {
    return nil, fmt.Errorf("failed to get applied migrations: %w", err)
  }

  status := make([]map[string]interface{}, len(migrations))
  appliedMap := make(map[uint]Migration)
  for _, m := range appliedMigrations {
    appliedMap[m.Version] = m
  }

  for i, migration := range migrations {
    applied, isApplied := appliedMap[migration.Version]
    var appliedAt *time.Time
    if isApplied {
      appliedAt = &applied.AppliedAt
    }

    status[i] = map[string]interface{}{
      "version":     migration.Version,
      "name":        migration.Name,
      "description": migration.Description,
      "is_applied":  isApplied,
      "applied_at":  appliedAt,
    }
  }

  return status, nil
}

// AddMigration adds a new migration step to the migrations list
func AddMigration(step MigrationStep) error {
  // Validate required fields
  if step.Version == 0 {
    return fmt.Errorf("migration version is required")
  }
  if step.Name == "" {
    return fmt.Errorf("migration name is required")
  }
  if step.Up == nil {
    return fmt.Errorf("migration Up function is required")
  }
  if step.Down == nil {
    return fmt.Errorf("migration Down function is required")
  }

  // Validate version is unique and sequential
  for _, m := range migrations {
    if m.Version == step.Version {
      return fmt.Errorf("migration version %d already exists", step.Version)
    }
  }

  // Ensure versions are sequential
  if len(migrations) > 0 && step.Version != migrations[len(migrations)-1].Version+1 {
    return fmt.Errorf("migration version must be sequential, expected version %d",
      migrations[len(migrations)-1].Version+1)
  }

  migrations = append(migrations, step)
  return nil
}

// RollbackLastMigration rolls back the last applied migration
func RollbackLastMigration(db *gorm.DB) error {
  var lastMigration Migration
  if err := db.Order("version desc").First(&lastMigration).Error; err != nil {
    return fmt.Errorf("failed to get last migration: %w", err)
  }

  // Find the migration step
  var migrationStep MigrationStep
  for _, m := range migrations {
    if m.Version == lastMigration.Version {
      migrationStep = m
      break
    }
  }

  if migrationStep.Down == nil {
    return fmt.Errorf("no down migration found for version %d", lastMigration.Version)
  }

  return db.Transaction(func(tx *gorm.DB) error {
    // Run the down migration
    if err := migrationStep.Down(tx); err != nil {
      return fmt.Errorf("failed to roll back migration %d: %w",
        lastMigration.Version, err)
    }

    // Soft delete the migration record
    if err := tx.Delete(&lastMigration).Error; err != nil {
      return fmt.Errorf("failed to delete migration record: %w", err)
    }

    fmt.Printf("Rolled back migration %d: %s\n", lastMigration.Version, lastMigration.Name)
    return nil
  })
}
