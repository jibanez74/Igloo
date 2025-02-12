package session

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gomodule/redigo/redis"
)

const (
	KeyUserID  = "user_id"
	KeyIsAdmin = "is_admin"
)

type Session interface {
	CreateAuthSession(*fiber.Ctx, *AuthData) error
	DestroyAuthSession(*fiber.Ctx) error
	IsAuthenticated(*fiber.Ctx) bool
	GetUserID(*fiber.Ctx) (int32, error)
	IsAdmin(*fiber.Ctx) bool
	AuthMiddleware() fiber.Handler
	Cleanup() error
}

type AuthData struct {
	ID      int32
	IsAdmin bool
}

type session struct {
	store *scs.SessionManager
}

func New(secure bool, redisPool *redis.Pool) *session {
	s := scs.New()
	s.Store = redisstore.New(redisPool)

	s.Lifetime = 24 * time.Hour
	s.Cookie.Persist = true
	s.Cookie.SameSite = http.SameSiteLaxMode
	s.Cookie.Secure = secure
	s.Cookie.Name = "igloo_session"

	return &session{store: s}
}

func (s *session) CreateAuthSession(c *fiber.Ctx, authData *AuthData) error {
	if authData == nil {
		return errors.New("auth data cannot be nil")
	}

	s.store.Put(c.Context(), KeyUserID, authData.ID)
	s.store.Put(c.Context(), KeyIsAdmin, authData.IsAdmin)

	return nil
}

func (s *session) DestroyAuthSession(c *fiber.Ctx) error {
	if err := s.store.RenewToken(c.Context()); err != nil {
		return fmt.Errorf("failed to renew token: %w", err)
	}

	if err := s.store.Destroy(c.Context()); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}

	return nil
}

func (s *session) IsAuthenticated(c *fiber.Ctx) bool {
	return s.store.Exists(c.Context(), KeyUserID)
}

func (s *session) GetUserID(c *fiber.Ctx) (int32, error) {
	id, ok := s.store.Get(c.Context(), KeyUserID).(int32)
	if !ok {
		return 0, errors.New("user ID not found in session")
	}
	return id, nil
}

func (s *session) IsAdmin(c *fiber.Ctx) bool {
	isAdmin, ok := s.store.Get(c.Context(), KeyIsAdmin).(bool)
	return ok && isAdmin
}

func (s *session) AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !s.IsAuthenticated(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		return c.Next()
	}
}

func (s *session) Cleanup() error {
	// Currently no cleanup needed for redis store
	// This method is added for future use and interface completeness
	return nil
}
