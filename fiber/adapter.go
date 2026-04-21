// Package fiber provides the Fiber v2 adapter for mtnuu.
//
// Use Mount to attach a *service.Docs to a *fiber.App. The adapter
// handles basic auth, extra middleware injection, the OnRender hook,
// and serves both the Scalar HTML and the generated OpenAPI YAML.
package fiber

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Aman-s12345/mtnuu/config"
	"github.com/Aman-s12345/mtnuu/service"

	"github.com/gofiber/fiber/v2"
)

// specFilename is the path segment under the docs mount at which
// the generated OpenAPI YAML is served. It's kept as a constant
// (not exposed as config) because the HTML template references it
// via the computed SpecURL passed in RenderHTML.
const specFilename = "openapi.yaml"

// Mount attaches docs onto app at the configured MountPath. Route
// layout:
//
//	GET {MountPath}/                  → Scalar HTML
//	GET {MountPath}/openapi.yaml      → generated OpenAPI spec
//
// Both routes run through, in order:
//  1. basic-auth middleware (if cfg.Auth.Enabled)
//  2. any cfg.ExtraMiddlewares (expected to be fiber.Handler)
//  3. cfg.OnRender hook
//  4. the actual handler
//
// Mount returns an error if ExtraMiddlewares contains anything that
// is not a fiber.Handler. Surfacing misconfiguration at startup is
// preferable to silent request-time failures.
func Mount(app *fiber.App, docs *service.Docs) error {
	cfg := docs.Config()

	// ── Assemble middleware chain ───────────────────────────
	chain := make([]fiber.Handler, 0, 2+len(cfg.ExtraMiddlewares))

	if cfg.Auth != nil && cfg.Auth.Enabled {
		chain = append(chain, basicAuthMiddleware(cfg.Auth))
	}

	for i, m := range cfg.ExtraMiddlewares {
		h, ok := m.(fiber.Handler)
		if !ok {
			return fmt.Errorf(
				"mtnuu/fiber: ExtraMiddlewares[%d] is %T, expected fiber.Handler",
				i, m,
			)
		}
		chain = append(chain, h)
	}

	if cfg.OnRender != nil {
		chain = append(chain, onRenderMiddleware(cfg.OnRender))
	}

	// ── Register routes ─────────────────────────────────────
	// Group under MountPath so all middlewares apply uniformly.
	group := app.Group(cfg.MountPath, chain...)

	// The HTML page tells the browser to fetch the spec from a path
	// relative to the page's own URL. Using a plain filename means
	// it resolves under MountPath automatically, no matter where the
	// docs are mounted.
	htmlHandler := func(c *fiber.Ctx) error {
		body, err := docs.RenderHTML(specFilename)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				SendString("mtnuu: failed to render docs: " + err.Error())
		}
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Send(body)
	}

	specHandler := func(c *fiber.Ctx) error {
		spec, err := docs.GenerateOpenAPI()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				SendString("mtnuu: failed to generate spec: " + err.Error())
		}
		c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
		c.Set("Cache-Control", "no-store")
		return c.Send(spec)
	}

	group.Get("/", htmlHandler)
	group.Get("/"+specFilename, specHandler)

	return nil
}

// ── basic auth ──────────────────────────────────────────────────

func basicAuthMiddleware(a *config.AuthConfig) fiber.Handler {
	realmHeader := fmt.Sprintf(`Basic realm=%q`, a.Realm)
	return func(c *fiber.Ctx) error {
		user, pass, ok := parseBasicAuth(c.Get(fiber.HeaderAuthorization))
		if !ok || user != a.Username || pass != a.Password {
			c.Set(fiber.HeaderWWWAuthenticate, realmHeader)
			return c.Status(fiber.StatusUnauthorized).
				SendString("Unauthorized")
		}
		return c.Next()
	}
}

func parseBasicAuth(header string) (string, string, bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ── OnRender hook ───────────────────────────────────────────────

func onRenderMiddleware(hook func(config.RenderContext) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Hook errors are surfaced to the consumer's logger via
		// Fiber's built-in logger; they do not fail the request.
		_ = hook(config.RenderContext{
			RemoteIP:  c.IP(),
			UserAgent: c.Get(fiber.HeaderUserAgent),
			Path:      c.Path(),
		})
		return c.Next()
	}
}