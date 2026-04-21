// Example application demonstrating mtnuu end-to-end.
//
// Run with:
//
//	go run ./example
//
// Then visit http://localhost:3000/docs  (user: admin, pass: secret).
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/Aman-s12345/mtnuu/config"
	mtnuufiber "github.com/Aman-s12345/mtnuu/fiber"
	"github.com/Aman-s12345/mtnuu/sdk"
	"github.com/Aman-s12345/mtnuu/service"

	"github.com/gofiber/fiber/v2"
)

// ── Example domain types ─────────────────────────────────────────
// These are the sort of SDK types a consumer would share between
// their handlers and their OpenAPI documentation.

type GetUserResponse struct {
	Success bool   `json:"success"`
	Name    string `json:"name"`
	Email   string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

func main() {
	// ── 1. Build the docs instance ──────────────────────────
	cfg := &config.Config{
		MountPath:   "/docs",
		DocName:     "Example",
		Description: "Demo API powered by mtnuu.",
		Version:     "0.1.0",
		MadeBy:      "Built with mtnuu",
		Theme:       "bluePlanet",
		DarkMode:    true,

		Servers: []config.Server{
			{URL: "http://localhost:3000", Description: "Local", Mode: "local"},
		},

		Auth: &config.AuthConfig{
			Enabled:  true,
			Username: "admin",
			Password: "secret",
		},

		// Custom middleware injection — stored as []any because the
		// core config is framework-agnostic. For the Fiber adapter,
		// each entry must be a fiber.Handler. Here we tag every docs
		// response with a request-id header.
		ExtraMiddlewares: []any{
			fiber.Handler(func(c *fiber.Ctx) error {
				c.Set("X-Docs-Request-ID",
					time.Now().Format("20060102T150405.000"))
				return c.Next()
			}),
		},

		// Custom hook fired on every docs page and spec fetch. Useful
		// for audit trails. Returning an error here is non-fatal — the
		// adapter just logs it.
		OnRender: func(rc config.RenderContext) error {
			log.Printf("docs accessed: ip=%s ua=%q path=%s",
				rc.RemoteIP, rc.UserAgent, rc.Path)
			return nil
		},
	}

	docs, err := service.New(cfg)
	if err != nil {
		log.Fatalf("mtnuu: %v", err)
	}

	// ── 2. Build the HTTP app ───────────────────────────────
	app := fiber.New()

	// Register actual routes.
	app.Get("/api/v1/users/:id", func(c *fiber.Ctx) error {
		return c.JSON(GetUserResponse{
			Success: true,
			Name:    "Alice",
			Email:   "alice@example.com",
		})
	})
	app.Post("/api/v1/users", func(c *fiber.Ctx) error {
		return c.JSON(CreateUserResponse{Success: true, ID: "usr_123"})
	})

	// ── 3. Register each route's OpenAPI metadata ───────────
	docs.Register(sdk.ApiWrapper{
		Path:        "/api/v1/users/:id",
		Method:      http.MethodGet,
		Name:        "Get User",
		Description: "Fetches a user by ID.",
		Tags:        []string{"Users"},
		Parameters: []sdk.ApiParameter{
			{Name: "id", In: "path", Description: "User ID", Required: true},
		},
		Response: &sdk.ApiResponse{
			Description: "User details",
			Content:     new(GetUserResponse),
		},
	})

	docs.Register(sdk.ApiWrapper{
		Path:        "/api/v1/users",
		Method:      http.MethodPost,
		Name:        "Create User",
		Description: "Creates a new user.",
		Tags:        []string{"Users"},
		RequestBody: &sdk.ApiRequestBody{
			Description: "User to create",
			Content:     new(CreateUserRequest),
		},
		Response: &sdk.ApiResponse{
			Description: "Created user",
			Content:     new(CreateUserResponse),
		},
	})

	// ── 4. Mount the docs UI onto the fiber app ─────────────
	if err := mtnuufiber.Mount(app, docs); err != nil {
		log.Fatalf("mtnuu mount: %v", err)
	}

	log.Printf("Registered %d operations", docs.Count())
	log.Println("Docs at http://localhost:3000/docs  (admin / secret)")
	log.Fatal(app.Listen(":3000"))
}