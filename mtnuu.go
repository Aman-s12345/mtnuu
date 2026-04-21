// Package mtnuu is a self-running OpenAPI spec generator and Scalar
// documentation host for Go web services.
//
// It lets each feature package declare its own OpenAPI metadata
// inline with its route definitions and collects everything into a
// single, live-served docs site without a separate build step.
//
// # Quick start
//
//	import (
//	    "github.com/Aman-s12345/mtnuu"
//	    "github.com/Aman-s12345/mtnuu/config"
//	    "github.com/Aman-s12345/mtnuu/sdk"
//	    mtnuufiber "github.com/Aman-s12345/mtnuu/fiber"
//	)
//
//	docs, err := mtnuu.New(&config.Config{
//	    DocName:  "Orders",
//	    Version:  "2.1.0",
//	    Servers:  []config.Server{{URL: "http://localhost:3000"}},
//	    Auth:     &config.AuthConfig{Enabled: true, Username: "u", Password: "p"},
//	})
//	if err != nil { log.Fatal(err) }
//
//	docs.Register(sdk.ApiWrapper{
//	    Path: "/api/v1/orders", Method: "GET",
//	    Name: "List orders", Tags: []string{"Orders"},
//	    Response: &sdk.ApiResponse{Content: new(OrdersResponse)},
//	})
//
//	app := fiber.New()
//	mtnuufiber.Mount(app, docs)
//	app.Listen(":3000")
//
// The docs site is then served at whatever config.MountPath resolves
// to (default: /docs), with the generated OpenAPI YAML at
// {MountPath}/openapi.yaml.
package mtnuu

import (
	"github.com/Aman-s12345/mtnuu/config"
	"github.com/Aman-s12345/mtnuu/sdk"
	"github.com/Aman-s12345/mtnuu/service"
)

// Docs is an alias for *service.Docs so consumers don't have to
// import the service subpackage just to hold a reference.
type Docs = service.Docs

// Config is an alias for *config.Config.
type Config = config.Config

// ApiWrapper is an alias for sdk.ApiWrapper.
type ApiWrapper = sdk.ApiWrapper

// New constructs a new Docs instance. Pass nil for defaults.
func New(cfg *config.Config) (*Docs, error) {
	return service.New(cfg)
}