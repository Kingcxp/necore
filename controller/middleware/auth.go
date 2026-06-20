package middleware

import (
	"necore/config"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
)

func AuthNeeded() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey:   jwtware.SigningKey{Key: []byte(config.Config("SECRET"))},
		ErrorHandler: jwtError,
		SuccessHandler: func(c *fiber.Ctx) error {
			return validateTokenVersion(c)
		},
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="necore"`)

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": "Unauthorized",
	})
}
