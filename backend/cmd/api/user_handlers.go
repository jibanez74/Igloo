package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getUserByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse user id from url params",
		})
	}

	user, err := app.queries.GetUserByID(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "unable to get user from database",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": user,
	})
}

func (app *application) createUser(c *fiber.Ctx) error {
	var req database.CreateUserParams

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse request body",
		})
	}

	exist, err := app.queries.CheckUserExists(c.Context(), database.CheckUserExistsParams{
		Email:    req.Email,
		Username: req.Username,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to check if user exists",
		})
	}

	if exist {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "user already exists",
		})
	}

	if len(req.Name) > 60 || len(req.Name) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name must be at least 2 characters and less than 60 characters",
		})
	}

	if len(req.Password) > 128 || len(req.Password) < 9 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password must be at least 9 characters and less than 128 characters",
		})
	}

	hash, err := helpers.HashPassword(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to hash password",
		})
	}

	req.Password = hash

	user, err := app.queries.CreateUser(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": user,
	})
}

func (app *application) getUsersPaginated(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset := (page - 1) * limit

	users, err := app.queries.GetUsersPaginated(c.Context(), database.GetUsersPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	count, err := app.queries.GetTotalUsersCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	// Transform users to remove sensitive information
	var safeUsers []fiber.Map
	for _, user := range users {
		safeUsers = append(safeUsers, fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
			"isActive": user.IsActive,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items":        safeUsers,
		"current_page": page,
		"total_pages":  totalPages,
		"count":        count,
	})
}

func (app *application) updateUserAvatar(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse user id from url params",
		})
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to get avatar file from request",
		})
	}

	resultChan := make(chan struct {
		path string
		err  error
	})

	go func() {
		imagePath, err := helpers.SaveAvatar(file, app.settings.AvatarImgDir)
		resultChan <- struct {
			path string
			err  error
		}{imagePath, err}
	}()

	result := <-resultChan
	if result.err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": result.err.Error(),
		})
	}

	user, err := app.queries.UpdateUserAvatar(c.Context(), database.UpdateUserAvatarParams{
		Avatar: result.path,
		ID:     int32(id),
	})

	if err != nil {
		// If database update fails, we should clean up the saved file
		os.Remove(result.path)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to update user avatar in database",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": user,
	})
}

func (app *application) deleteUser(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse user id from url params",
		})
	}

	err = app.queries.DeleteUser(c.Context(), int32(id))
	if err != nil {
		if err.Error() == "pq: cannot delete the last admin user" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "cannot delete the last admin user",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to delete user",
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
