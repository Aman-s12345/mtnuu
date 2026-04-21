# mtnuu

A self-running OpenAPI spec generator and Scalar documentation host for Go web services.

Declare OpenAPI metadata inline with your route definitions. mtnuu collects it
all into a single, live-served docs site — no separate build step, no YAML to
commit, no asset files to ship.

Currently supports **Fiber v2**. The core is framework-agnostic; new adapters
can be added under their own subpackage without touching the rest.

---

## Install

```bash
go get github.com/Aman-s12345/mtnuu
```

## Quick start

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v2"

    "github.com/Aman-s12345/mtnuu"
    "github.com/Aman-s12345/mtnuu/config"
    "github.com/Aman-s12345/mtnuu/sdk"
    mtnuufiber "github.com/Aman-s12345/mtnuu/fiber"
)

type User struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func main() {
    app := fiber.New()

    docs, err := mtnuu.New(&config.Config{
        MountPath: "/docs",
        DocName:   "Users",
        Version:   "1.0.0",
        Servers: []config.Server{
            {URL: "http://localhost:3000", Description: "Local"},
        },
        Auth: &config.AuthConfig{
            Enabled: true, Username: "admin", Password: "secret",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    app.Get("/api/v1/users/:id", func(c *fiber.Ctx) error {
        return c.JSON(User{ID: c.Params("id"), Email: "a@b.com"})
    })

    docs.Register(sdk.ApiWrapper{
        Path: "/api/v1/users/:id", Method: "GET",
        Name: "Get user by ID",
        Tags: []string{"Users"},
        Parameters: []sdk.ApiParameter{
            {Name: "id", In: "path", Required: true},
        },
        Response: &sdk.ApiResponse{Content: new(User)},
    })

    if err := mtnuufiber.Mount(app, docs); err != nil {
        log.Fatal(err)
    }
    log.Fatal(app.Listen(":3000"))
}
```

Open `http://localhost:3000/docs`, log in with `admin` / `secret`, and the
Scalar UI appears with your route fully documented. The raw OpenAPI spec is at
`http://localhost:3000/docs/openapi.yaml`.

---

## Package layout

```
mtnuu/
├── mtnuu.go       → top-level re-exports (mtnuu.New, mtnuu.Docs, mtnuu.Config)
├── config/        → Config, Server, AuthConfig, RenderContext
├── sdk/           → ApiWrapper, ApiParameter, ApiRequestBody, ApiResponse
├── service/       → framework-agnostic core (OpenAPI gen + HTML render)
├── fiber/         → Fiber v2 adapter — Mount(app, docs)
└── example/       → runnable demo
```

The separation is deliberate: your feature packages import only `sdk` when
declaring routes, so they don't pull in the HTML template or Fiber.

---

## Config reference

| Field              | Type                            | Default                | Notes                                                               |
| ------------------ | ------------------------------- | ---------------------- | ------------------------------------------------------------------- |
| `MountPath`        | `string`                        | `/docs`                | Leading slash added if missing; trailing slash stripped.            |
| `DocName`          | `string`                        | `API`                  | Shown in header, browser title, IndexedDB namespace.                |
| `Title`            | `string`                        | `{DocName} API Documentation` | Full page title.                                              |
| `Description`      | `string`                        | `{DocName} API documentation.` | OpenAPI `info.description`.                                 |
| `Version`          | `string`                        | `1.0.0`                | OpenAPI `info.version`.                                             |
| `LogoURL`          | `string`                        | empty                  | Path or absolute URL to a logo image. Hidden if empty.              |
| `MadeBy`           | `string`                        | empty                  | Attribution string on the right of the header. Hidden if empty.     |
| `Theme`            | `string`                        | `bluePlanet`           | Scalar theme name.                                                  |
| `DarkMode`         | `bool`                          | `false`                | Force dark mode; otherwise Scalar follows OS preference.            |
| `Servers`          | `[]config.Server`               | empty                  | Exposed in the OpenAPI `servers` block and in the Scalar switcher.  |
| `Auth`             | `*config.AuthConfig`            | nil                    | Basic auth; set `Enabled: true` to gate access.                     |
| `ExtraMiddlewares` | `[]any`                         | nil                    | Framework-specific middleware stack. Fiber adapter expects `fiber.Handler`. |
| `OnRender`         | `func(RenderContext) error`     | nil                    | Called on every docs page and spec fetch. Errors are logged, not blocking. |

### `Server`

```go
type Server struct {
    URL         string // e.g. "https://api.example.com"
    Description string
    Mode        string // optional environment tag ("local", "uat", "prod")
}
```

Each server's host is injected into the Scalar page as a JS array, so the
built-in request-log sidebar knows which requests to capture.

### `AuthConfig`

```go
type AuthConfig struct {
    Enabled  bool
    Username string
    Password string
    Realm    string // default: "{DocName} Docs"
}
```

---

## Features

### Custom middleware injection

Inject any framework middleware before the docs handler — rate limiting,
IP allow-list, audit logging, feature flags:

```go
ExtraMiddlewares: []any{
    fiber.Handler(func(c *fiber.Ctx) error {
        if !isAllowedIP(c.IP()) {
            return c.SendStatus(403)
        }
        return c.Next()
    }),
    limiter.New(),  // any off-the-shelf fiber middleware
},
```

Each entry must be a `fiber.Handler`; `Mount` returns an error at startup
otherwise.

### OnRender hook

Observability without having to write middleware:

```go
OnRender: func(rc config.RenderContext) error {
    metrics.DocsAccessCounter.Inc()
    log.Printf("docs access: ip=%s path=%s", rc.RemoteIP, rc.Path)
    return nil
},
```

The hook runs after auth + middlewares have passed. Returning an error only
logs — it never blocks the response.

### Multiple independent doc sites in one process

Unlike the original wealth_be implementation, mtnuu has no package-level state.
Create as many `*Docs` instances as you want — useful for mounting separate
admin-API and public-API documentation at different paths:

```go
publicDocs, _ := mtnuu.New(&config.Config{MountPath: "/docs", DocName: "Public API"})
adminDocs,  _ := mtnuu.New(&config.Config{MountPath: "/admin/docs", DocName: "Admin API"})

publicDocs.Register(...)
adminDocs.Register(...)

mtnuufiber.Mount(app, publicDocs)
mtnuufiber.Mount(app, adminDocs)
```

### Writing the spec to disk

For CI pipelines that want the generated YAML committed:

```go
if err := docs.WriteOpenAPIFile("openapi.yaml"); err != nil {
    log.Fatal(err)
}
```

---

## Routes served under `MountPath`

| Route                    | Serves                          |
| ------------------------ | ------------------------------- |
| `GET {MountPath}/`       | Scalar HTML documentation page  |
| `GET {MountPath}/openapi.yaml` | Generated OpenAPI 3.0.3 YAML |

Both routes go through the same chain: basic auth (if enabled) →
`ExtraMiddlewares` → `OnRender` hook → handler.

---


## What's not (yet) built

- Adapters for Gin, Echo, net/http — the core is ready for them; PRs welcome.
- Spec caching — `GenerateOpenAPI` reflects on every call. Fine up to a few
  hundred operations; if yours hits thousands, we'll add a cache.
- Automatic tag inference — tags are supplied per `ApiWrapper`.

---

## License

TBD by the maintainer.