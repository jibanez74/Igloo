package main

import (
	"igloo/cmd/internal/database"
	"sync"
)

// movieScannerCache holds cached artists and production companies during movie scanning
// to reduce TMDB API calls for frequently repeated entities.
type movieScannerCache struct {
	artists            map[int]*database.Artist
	productionCompanies map[int]*database.ProductionCompany
	mu                 sync.RWMutex
}

// newMovieScannerCache creates a new empty cache for movie scanning.
func newMovieScannerCache() *movieScannerCache {
	return &movieScannerCache{
		artists:             make(map[int]*database.Artist),
		productionCompanies: make(map[int]*database.ProductionCompany),
	}
}

// GetArtist retrieves a cached artist by TMDB ID.
// Returns the artist and true if found, nil and false otherwise.
func (c *movieScannerCache) GetArtist(tmdbID int) (*database.Artist, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	artist, ok := c.artists[tmdbID]
	return artist, ok
}

// SetArtist caches an artist by TMDB ID.
func (c *movieScannerCache) SetArtist(tmdbID int, artist *database.Artist) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artists[tmdbID] = artist
}

// GetProductionCompany retrieves a cached production company by TMDB ID.
// Returns the company and true if found, nil and false otherwise.
func (c *movieScannerCache) GetProductionCompany(tmdbID int) (*database.ProductionCompany, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	company, ok := c.productionCompanies[tmdbID]
	return company, ok
}

// SetProductionCompany caches a production company by TMDB ID.
func (c *movieScannerCache) SetProductionCompany(tmdbID int, company *database.ProductionCompany) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.productionCompanies[tmdbID] = company
}

// Clear removes all cached data.
// Should be called after scan completes to free memory.
func (c *movieScannerCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artists = make(map[int]*database.Artist)
	c.productionCompanies = make(map[int]*database.ProductionCompany)
}
