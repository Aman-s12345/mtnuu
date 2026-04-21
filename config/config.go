// Package config holds user-facing configuration for mtnuu.
//
// Consumers build a *Config, pass it to service.New, and mount the
// resulting *Docs on their HTTP framework via a subpackage adapter
// (e.g. github.com/Aman-s12345/mtnuu/fiber).
package config

import (
	"errors"
	"strings"
)

// Config describes how a Docs instance should behave at runtime.
//
// All fields have sensible defaults applied by Normalize, so the
// zero value Config{} is valid and will produce an unauthenticated
// docs page mounted at /docs.
type Config struct {
	// ── Mount ───────────────────────────────────────────────

	// MountPath is the URL prefix where the docs UI is served.
	// Default: "/docs".
	MountPath string

	// ── Branding ────────────────────────────────────────────

	// DocName is the product/API name shown in the header,
	// browser title, and all "made by X" style strings.
	// Default: "API".
	DocName string

	// Title is the full page title. If empty, it is derived as
	// "{DocName} API Documentation".
	Title string

	// Description is the long-form description shown in the
	// OpenAPI info block. Default: "API documentation.".
	Description string

	// Version is the API version string. Default: "1.0.0".
	Version string

	// LogoURL is an optional path or absolute URL to a logo image
	// rendered in the custom header. If empty, only the text logo
	// shows. A path like "/docs/logo.png" works if the consumer
	// serves that asset themselves.
	LogoURL string

	// MadeBy is the attribution text shown on the right of the
	// header. Empty string hides the element entirely.
	MadeBy string

	// ── Scalar UI ───────────────────────────────────────────

	// Theme is the Scalar theme name, e.g. "bluePlanet", "default",
	// "moon", "solarized". Default: "bluePlanet".
	Theme string

	// DarkMode forces dark mode on. Default: false (Scalar picks
	// based on the user's system preference). Set true for a site
	// that is always dark regardless of OS setting.
	DarkMode bool

	// ── OpenAPI ─────────────────────────────────────────────

	// Servers is the list of servers exposed in the OpenAPI doc
	// and offered in the Scalar server switcher.
	Servers []Server

	// ── Auth ────────────────────────────────────────────────

	// Auth gates access to the docs UI. When nil or disabled, the
	// docs are served without authentication.
	Auth *AuthConfig

	// ExtraMiddlewares lets the consumer inject any additional
	// middleware (rate limiting, IP allow-list, custom logging,
	// feature flags, etc.) ahead of the docs handler. Each value
	// is framework-specific — the Fiber adapter expects fiber.Handler.
	//
	// Stored as []any so the core package does not depend on any
	// particular web framework.
	ExtraMiddlewares []any

	// OnRender is an optional hook called once per docs page render,
	// giving the consumer a chance to observe or audit access. It
	// runs after auth + middlewares have passed. Errors are logged
	// by the adapter but do not block the render.
	OnRender func(RenderContext) error
}

// Server describes one server entry in the OpenAPI "servers" block.
// Mode is an opaque string the Scalar page uses to flag the "live"
// environment, mirroring the FINZOOM_DOCS_MODE pattern from the
// original HTML. Leave Mode empty if you do not use per-env tagging.
type Server struct {
	URL         string
	Description string
	Mode        string // optional environment tag, e.g. "local", "uat", "prod"
}

// AuthConfig configures basic auth on the docs route.
type AuthConfig struct {
	// Enabled, when false, disables auth even if Username/Password
	// are set. Makes it easy to flip auth off via env flag without
	// clearing credentials.
	Enabled bool

	// Username and Password are the basic auth credentials. Both
	// must be non-empty when Enabled is true.
	Username string
	Password string

	// Realm is the WWW-Authenticate realm shown in the browser
	// password prompt. Default: "{DocName} Docs".
	Realm string
}

// RenderContext is the information passed to Config.OnRender.
type RenderContext struct {
	// RemoteIP is the client's IP address as reported by the
	// framework. May be an empty string if the framework does not
	// expose it cleanly.
	RemoteIP string
	// UserAgent is the User-Agent header value.
	UserAgent string
	// Path is the path that was requested inside the docs mount.
	Path string
}

// Normalize applies defaults to unset fields and validates the config.
// It is called automatically by service.New, but consumers may call
// it directly to surface configuration errors at startup.
func (c *Config) Normalize() error {
	if c == nil {
		return errors.New("mtnuu: config is nil")
	}

	if c.MountPath == "" {
		c.MountPath = "/docs"
	}
	// Ensure leading slash, strip trailing slash (but keep "/").
	if !strings.HasPrefix(c.MountPath, "/") {
		c.MountPath = "/" + c.MountPath
	}
	if len(c.MountPath) > 1 && strings.HasSuffix(c.MountPath, "/") {
		c.MountPath = strings.TrimRight(c.MountPath, "/")
	}

	if c.DocName == "" {
		c.DocName = "API"
	}
	if c.Title == "" {
		c.Title = c.DocName + " API Documentation"
	}
	if c.Description == "" {
		c.Description = c.DocName + " API documentation."
	}
	if c.Version == "" {
		c.Version = "1.0.0"
	}
	if c.Theme == "" {
		c.Theme = "bluePlanet"
	}

	if c.Auth != nil && c.Auth.Enabled {
		if c.Auth.Username == "" || c.Auth.Password == "" {
			return errors.New("mtnuu: auth enabled but username/password empty")
		}
		if c.Auth.Realm == "" {
			c.Auth.Realm = c.DocName + " Docs"
		}
	}

	return nil
}