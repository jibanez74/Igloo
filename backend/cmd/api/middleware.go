package main

import "github.com/gofiber/fiber/v2"

func (app *application) validateSession(c *fiber.Ctx) error {
	return c.Next()
}
