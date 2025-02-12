package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/valyala/fasthttp/fasthttpadaptor"
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

const (
	KeyUserID  = "user_id"
	KeyIsAdmin = "is_admin"
)

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

func (s *session) getContext(c *fiber.Ctx) context.Context {
	var r http.Request

	fasthttpadaptor.ConvertRequest(c.Context(), &r, true)

	token := c.Cookies(s.store.Cookie.Name)

	ctx, _ := s.store.Load(r.Context(), token)

	return ctx
}

func (s *session) CreateAuthSession(c *fiber.Ctx, authData *AuthData) error {
	if authData == nil {
		return errors.New("auth data cannot be nil")
	}

	ctx := s.getContext(c)

	s.store.Put(ctx, KeyUserID, authData.ID)
	s.store.Put(ctx, KeyIsAdmin, authData.IsAdmin)

	// Get token and save session data
	token, expiry, err := s.store.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit session: %w", err)
	}

	// Set the session cookie
	cookie := &fiber.Cookie{
		Name:     s.store.Cookie.Name,
		Value:    token,
		Expires:  expiry,
		Secure:   s.store.Cookie.Secure,
		HTTPOnly: true,
	}

	// Convert http.SameSite to string
	switch s.store.Cookie.SameSite {
	case http.SameSiteLaxMode:
		cookie.SameSite = "Lax"
	case http.SameSiteStrictMode:
		cookie.SameSite = "Strict"
	case http.SameSiteNoneMode:
		cookie.SameSite = "None"
	default:
		cookie.SameSite = "Lax"
	}

	c.Cookie(cookie)
	return nil
}

func (s *session) DestroyAuthSession(c *fiber.Ctx) error {
	ctx := s.getContext(c)

	if err := s.store.RenewToken(ctx); err != nil {
		return fmt.Errorf("failed to renew token: %w", err)
	}

	if err := s.store.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}

	// Clear the session cookie
	c.Cookie(&fiber.Cookie{
		Name:     s.store.Cookie.Name,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return nil
}

func (s *session) IsAuthenticated(c *fiber.Ctx) bool {
	ctx := s.getContext(c)

	return s.store.Exists(ctx, KeyUserID)
}

func (s *session) GetUserID(c *fiber.Ctx) (int32, error) {
	ctx := s.getContext(c)
	id, ok := s.store.Get(ctx, KeyUserID).(int32)
	if !ok {
		return 0, errors.New("user ID not found in session")
	}
	return id, nil
}

func (s *session) IsAdmin(c *fiber.Ctx) bool {
	ctx := s.getContext(c)
	isAdmin, ok := s.store.Get(ctx, KeyIsAdmin).(bool)
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
	return nil
}
