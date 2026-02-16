package main

import (
	"sync"
	"testing"

	"igloo/cmd/internal/database"
)

func TestNewMovieScannerCache(t *testing.T) {
	cache := newMovieScannerCache()

	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}

	if cache.artists == nil {
		t.Error("Expected artists map to be initialized")
	}

	if cache.productionCompanies == nil {
		t.Error("Expected productionCompanies map to be initialized")
	}

	// Verify maps are empty
	if len(cache.artists) != 0 {
		t.Errorf("Expected empty artists map, got %d entries", len(cache.artists))
	}

	if len(cache.productionCompanies) != 0 {
		t.Errorf("Expected empty productionCompanies map, got %d entries", len(cache.productionCompanies))
	}
}

func TestCacheGetArtist_SetArtist(t *testing.T) {
	cache := newMovieScannerCache()

	t.Run("get non-existent artist returns nil, false", func(t *testing.T) {
		artist, ok := cache.GetArtist(12345)
		if ok {
			t.Error("Expected false for non-existent artist")
		}
		if artist != nil {
			t.Errorf("Expected nil artist, got %v", artist)
		}
	})

	t.Run("set and get artist returns artist, true", func(t *testing.T) {
		tmdbID := 12345
		artist := &database.Artist{
			ID:    1,
			Name:  "Test Artist",
			TmdbID: int64(tmdbID),
		}

		cache.SetArtist(tmdbID, artist)

		retrieved, ok := cache.GetArtist(tmdbID)
		if !ok {
			t.Error("Expected true for existing artist")
		}
		if retrieved == nil {
			t.Fatal("Expected non-nil artist")
		}
		if retrieved.ID != artist.ID {
			t.Errorf("Expected artist ID %d, got %d", artist.ID, retrieved.ID)
		}
		if retrieved.Name != artist.Name {
			t.Errorf("Expected artist name %q, got %q", artist.Name, retrieved.Name)
		}
	})

	t.Run("overwrite existing artist returns new artist", func(t *testing.T) {
		tmdbID := 12345
		firstArtist := &database.Artist{
			ID:    1,
			Name:  "First Artist",
			TmdbID: int64(tmdbID),
		}

		secondArtist := &database.Artist{
			ID:    2,
			Name:  "Second Artist",
			TmdbID: int64(tmdbID),
		}

		cache.SetArtist(tmdbID, firstArtist)
		cache.SetArtist(tmdbID, secondArtist)

		retrieved, ok := cache.GetArtist(tmdbID)
		if !ok {
			t.Fatal("Expected artist to exist")
		}
		if retrieved.ID != secondArtist.ID {
			t.Errorf("Expected second artist ID %d, got %d", secondArtist.ID, retrieved.ID)
		}
		if retrieved.Name != secondArtist.Name {
			t.Errorf("Expected second artist name %q, got %q", secondArtist.Name, retrieved.Name)
		}
	})
}

func TestCacheGetProductionCompany_SetProductionCompany(t *testing.T) {
	cache := newMovieScannerCache()

	t.Run("get non-existent company returns nil, false", func(t *testing.T) {
		company, ok := cache.GetProductionCompany(12345)
		if ok {
			t.Error("Expected false for non-existent company")
		}
		if company != nil {
			t.Errorf("Expected nil company, got %v", company)
		}
	})

	t.Run("set and get company returns company, true", func(t *testing.T) {
		tmdbID := 12345
		company := &database.ProductionCompany{
			ID:    1,
			Name:  "Test Company",
			TmdbID: int64(tmdbID),
		}

		cache.SetProductionCompany(tmdbID, company)

		retrieved, ok := cache.GetProductionCompany(tmdbID)
		if !ok {
			t.Error("Expected true for existing company")
		}
		if retrieved == nil {
			t.Fatal("Expected non-nil company")
		}
		if retrieved.ID != company.ID {
			t.Errorf("Expected company ID %d, got %d", company.ID, retrieved.ID)
		}
		if retrieved.Name != company.Name {
			t.Errorf("Expected company name %q, got %q", company.Name, retrieved.Name)
		}
	})

	t.Run("overwrite existing company returns new company", func(t *testing.T) {
		tmdbID := 12345
		firstCompany := &database.ProductionCompany{
			ID:    1,
			Name:  "First Company",
			TmdbID: int64(tmdbID),
		}

		secondCompany := &database.ProductionCompany{
			ID:    2,
			Name:  "Second Company",
			TmdbID: int64(tmdbID),
		}

		cache.SetProductionCompany(tmdbID, firstCompany)
		cache.SetProductionCompany(tmdbID, secondCompany)

		retrieved, ok := cache.GetProductionCompany(tmdbID)
		if !ok {
			t.Fatal("Expected company to exist")
		}
		if retrieved.ID != secondCompany.ID {
			t.Errorf("Expected second company ID %d, got %d", secondCompany.ID, retrieved.ID)
		}
		if retrieved.Name != secondCompany.Name {
			t.Errorf("Expected second company name %q, got %q", secondCompany.Name, retrieved.Name)
		}
	})
}

func TestCacheClear(t *testing.T) {
	cache := newMovieScannerCache()

	// Add some data
	artist := &database.Artist{ID: 1, Name: "Test Artist", TmdbID: 12345}
	company := &database.ProductionCompany{ID: 1, Name: "Test Company", TmdbID: 67890}

	cache.SetArtist(12345, artist)
	cache.SetProductionCompany(67890, company)

	// Verify data exists
	if len(cache.artists) != 1 {
		t.Errorf("Expected 1 artist before clear, got %d", len(cache.artists))
	}
	if len(cache.productionCompanies) != 1 {
		t.Errorf("Expected 1 company before clear, got %d", len(cache.productionCompanies))
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	if len(cache.artists) != 0 {
		t.Errorf("Expected 0 artists after clear, got %d", len(cache.artists))
	}
	if len(cache.productionCompanies) != 0 {
		t.Errorf("Expected 0 companies after clear, got %d", len(cache.productionCompanies))
	}

	// Verify can't retrieve cleared data
	_, ok := cache.GetArtist(12345)
	if ok {
		t.Error("Expected artist to be removed after clear")
	}

	_, ok = cache.GetProductionCompany(67890)
	if ok {
		t.Error("Expected company to be removed after clear")
	}

	// Verify can add new items after clear
	newArtist := &database.Artist{ID: 2, Name: "New Artist", TmdbID: 11111}
	cache.SetArtist(11111, newArtist)

	retrieved, ok := cache.GetArtist(11111)
	if !ok {
		t.Error("Expected to be able to add new artist after clear")
	}
	if retrieved.ID != newArtist.ID {
		t.Errorf("Expected new artist ID %d, got %d", newArtist.ID, retrieved.ID)
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := newMovieScannerCache()

	// Test concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		artist := &database.Artist{ID: 1, Name: "Concurrent Artist", TmdbID: 12345}
		cache.SetArtist(12345, artist)

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				retrieved, ok := cache.GetArtist(12345)
				if !ok {
					t.Error("Expected artist to be found in concurrent read")
				}
				if retrieved == nil {
					t.Error("Expected non-nil artist in concurrent read")
				}
			}()
		}

		wg.Wait()
	})

	// Test concurrent writes
	t.Run("concurrent writes", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				artist := &database.Artist{
					ID:    int64(id),
					Name:  "Concurrent Artist",
					TmdbID: int64(id),
				}
				cache.SetArtist(id, artist)
			}(i)
		}

		wg.Wait()

		// Verify all artists were set
		for i := 0; i < numGoroutines; i++ {
			_, ok := cache.GetArtist(i)
			if !ok {
				t.Errorf("Expected artist %d to be set", i)
			}
		}
	})

	// Test concurrent read and write
	t.Run("concurrent read and write", func(t *testing.T) {
		artist := &database.Artist{ID: 1, Name: "Read Write Artist", TmdbID: 99999}
		cache.SetArtist(99999, artist)

		var wg sync.WaitGroup
		numReaders := 5
		numWriters := 5

		// Start readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_, _ = cache.GetArtist(99999)
				}
			}()
		}

		// Start writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					newArtist := &database.Artist{
						ID:    int64(id*1000 + j),
						Name:  "Updated Artist",
						TmdbID: 99999,
					}
					cache.SetArtist(99999, newArtist)
				}
			}(i)
		}

		wg.Wait()

		// Verify final state
		retrieved, ok := cache.GetArtist(99999)
		if !ok {
			t.Error("Expected artist to exist after concurrent access")
		}
		if retrieved == nil {
			t.Error("Expected non-nil artist after concurrent access")
		}
	})
}
