package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tokens"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

const (
	authErr   = "invalid credentials"
	serverErr = "server error"
)

func (app *application) login(c *fiber.Ctx) error {
	var req loginRequest

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing required fields",
		})
	}

	user, err := app.queries.GetUserForLogin(c.Context(), database.GetUserForLoginParams{
		Email:    req.Email,
		Username: req.Username,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": authErr,
		})
	}

	match, err := helpers.PasswordMatches(req.Password, user.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	if !match {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	tokenPair, err := app.tokens.GenerateTokenPair(tokens.Claims{
		UserID:   int(user.ID),
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"tokens": tokenPair,
		"user": fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"avatar":   user.Avatar,
			"isAdmin":  user.IsAdmin,
		},
	})
}

func (app *application) requestDeviceCode(c *fiber.Ctx) error {
	deviceCode := helpers.GenerateRandomString(8)
	userCode := helpers.GenerateRandomString(8)
	expiresAt := time.Now().Add(10 * time.Minute)

	deviceCodeRecord, err := app.queries.CreateDeviceCode(c.Context(), database.CreateDeviceCodeParams{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		ExpiresAt:  pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to create device code: %v", err),
		})
	}

	verificationURI := fmt.Sprintf("%s/verify-device?code=%s", app.settings.BaseUrl, userCode)

	return c.JSON(fiber.Map{
		"device_code":      deviceCodeRecord.DeviceCode,
		"user_code":        deviceCodeRecord.UserCode,
		"verification_uri": verificationURI,
		"expires_in":       1800, // 30 minutes in seconds
	})
}

func (app *application) verifyUserCode(c *fiber.Ctx) error {
	var request struct {
		UserCode string `json:"user_code"`
	}

	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	userID := c.Locals("user_id").(int32)

	err = app.queries.VerifyDeviceCode(c.Context(), database.VerifyDeviceCodeParams{
		UserCode: request.UserCode,
		UserID:   pgtype.Int4{Int32: userID, Valid: true},
	})

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "invalid or expired user code",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "device code verified successfully",
	})
}

func (app *application) handleDeviceCodeWebSocket(c *fiber.Ctx) error {
	deviceCode := c.Params("device_code")
	if deviceCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "device code is required",
		})
	}

	ctx := c.Context()

	ws := websocket.New(func(c *websocket.Conn) {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				code, err := app.queries.GetDeviceCode(ctx, deviceCode)
				if err != nil {
					c.WriteJSON(fiber.Map{
						"error": "invalid or expired device code",
					})

					return
				}

				if code.IsVerified {
					tokenPair, err := app.tokens.GenerateTokenPair(tokens.Claims{
						UserID:   int(code.UserID.Int32),
						Username: "", // We don't need these for device tokens
						Email:    "",
						IsAdmin:  false,
					})

					if err != nil {
						c.WriteJSON(fiber.Map{
							"error": "failed to generate tokens",
						})

						return
					}

					c.WriteJSON(fiber.Map{
						"access_token":  tokenPair.AccessToken,
						"refresh_token": tokenPair.RefreshToken,
					})

					return
				}

				c.WriteJSON(fiber.Map{
					"message": "authorization pending",
				})
			}
		}()

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				c.WriteMessage(messageType, message)
			}
		}
	})

	return ws(c)
}
