// Package service is the framework-agnostic core of mtnuu.
//
// A *Docs holds a registry of ApiWrapper entries and knows how to
// emit them as an OpenAPI 3.0.3 YAML document, and how to render
// the Scalar UI HTML. Framework adapters (e.g. mtnuu/fiber) wrap
// a *Docs with concrete HTTP routes.
package service

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/Aman-s12345/mtnuu/config"
	"github.com/Aman-s12345/mtnuu/sdk"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"gopkg.in/yaml.v3"
)

// Docs is an isolated documentation instance. Unlike the original
// implementation (which used package-level state), each *Docs owns
// its own registry, so a single process can host multiple independent
// doc sets if needed.
type Docs struct {
	cfg *config.Config

	mu   sync.RWMutex
	apis map[string][]sdk.ApiWrapper

	tmpl *template.Template
}

// New builds a *Docs from cfg, applies defaults, and parses the
// embedded Scalar HTML template. It fails if the config is invalid.
func New(cfg *config.Config) (*Docs, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	tmpl, err := template.New("index.html").Parse(indexHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("mtnuu: parsing embedded template: %w", err)
	}

	return &Docs{
		cfg:  cfg,
		apis: map[string][]sdk.ApiWrapper{},
		tmpl: tmpl,
	}, nil
}

// Config returns the normalized config the Docs was built with.
// Adapters use this to read MountPath, Auth, ExtraMiddlewares, etc.
func (d *Docs) Config() *config.Config { return d.cfg }

// Register adds an API definition to the registry. Safe for
// concurrent use, though registration is expected to happen at
// startup before any serving begins.
func (d *Docs) Register(api sdk.ApiWrapper) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.apis[api.Path] = append(d.apis[api.Path], api)
}

// Count returns the total number of registered operations across
// all paths. Useful for startup sanity checks.
func (d *Docs) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := 0
	for _, ops := range d.apis {
		n += len(ops)
	}
	return n
}

// GenerateOpenAPI renders the registered APIs as an OpenAPI 3.0.3
// YAML document. Returns an error if no APIs have been registered
// or if schema reflection fails for any operation.
func (d *Docs) GenerateOpenAPI() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.apis) == 0 {
		return nil, errors.New("mtnuu: no APIs registered")
	}

	swagger := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       d.cfg.Title,
			Description: d.cfg.Description,
			Version:     d.cfg.Version,
		},
	}

	// Servers
	if len(d.cfg.Servers) > 0 {
		servers := make(openapi3.Servers, 0, len(d.cfg.Servers))
		for _, s := range d.cfg.Servers {
			servers = append(servers, &openapi3.Server{
				URL:         s.URL,
				Description: s.Description,
			})
		}
		swagger.Servers = servers
	}

	pathOpts := make([]openapi3.NewPathsOption, 0, len(d.apis))
	for _, ops := range d.apis {
		pathItem, err := d.buildPathItem(ops)
		if err != nil {
			return nil, fmt.Errorf("mtnuu: building path %q: %w", ops[0].Path, err)
		}
		pathOpts = append(pathOpts, openapi3.WithPath(rewritePathParams(ops[0].Path), pathItem))
	}
	swagger.Paths = openapi3.NewPaths(pathOpts...)

	// Security scheme — always define BearerAuth; operations opt in
	// individually via !UnAuthenticated.
	com := openapi3.NewComponents()
	com.SecuritySchemes = openapi3.SecuritySchemes{
		"BearerAuth": &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:         "http",
				Description:  "Bearer token authentication",
				Scheme:       "bearer",
				BearerFormat: "JWT",
			},
		},
	}
	swagger.Components = &com

	out, err := yaml.Marshal(swagger)
	if err != nil {
		return nil, fmt.Errorf("mtnuu: marshaling OpenAPI: %w", err)
	}
	return out, nil
}

// WriteOpenAPIFile is a convenience wrapper that writes the spec
// to disk. Useful for CI jobs that commit the generated YAML.
func (d *Docs) WriteOpenAPIFile(path string) error {
	data, err := d.GenerateOpenAPI()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// buildPathItem converts all operations sharing a path into a
// single openapi3.PathItem with per-method operations.
func (d *Docs) buildPathItem(ops []sdk.ApiWrapper) (*openapi3.PathItem, error) {
	item := &openapi3.PathItem{}

	for _, api := range ops {
		op := &openapi3.Operation{
			Summary:     api.Name,
			Description: api.Description,
			Tags:        api.Tags,
		}

		if api.RequestBody != nil {
			ref, err := buildRequestBody(*api.RequestBody)
			if err != nil {
				return nil, fmt.Errorf("request body for %s: %w", api.Name, err)
			}
			op.RequestBody = ref
		}

		if api.Response != nil {
			resps, err := buildResponses(*api.Response)
			if err != nil {
				return nil, fmt.Errorf("response for %s: %w", api.Name, err)
			}
			op.Responses = resps
		}

		if len(api.Parameters) > 0 {
			op.Parameters = buildParameters(api.Parameters)
		}

		if api.UnAuthenticated {
			op.Security = nil
		} else {
			op.Security = &openapi3.SecurityRequirements{
				{"BearerAuth": []string{}},
			}
		}

		switch strings.ToUpper(api.Method) {
		case "GET":
			item.Get = op
		case "POST":
			item.Post = op
		case "PUT":
			item.Put = op
		case "DELETE":
			item.Delete = op
		case "PATCH":
			item.Patch = op
		case "HEAD":
			item.Head = op
		case "OPTIONS":
			item.Options = op
		default:
			return nil, fmt.Errorf("unsupported HTTP method %q", api.Method)
		}
	}

	return item, nil
}

func buildRequestBody(rb sdk.ApiRequestBody) (*openapi3.RequestBodyRef, error) {
	if rb.Content == nil {
		return nil, errors.New("request body content must be provided")
	}
	schemaRef, err := openapi3gen.NewSchemaRefForValue(rb.Content, nil)
	if err != nil {
		return nil, fmt.Errorf("reflecting schema: %w", err)
	}
	return &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Description: rb.Description,
			Content: map[string]*openapi3.MediaType{
				"application/json": {Schema: schemaRef},
			},
		},
	}, nil
}

func buildResponses(r sdk.ApiResponse) (*openapi3.Responses, error) {
	if r.Content == nil {
		return nil, errors.New("response content must be provided")
	}
	schemaRef, err := openapi3gen.NewSchemaRefForValue(r.Content, nil)
	if err != nil {
		return nil, fmt.Errorf("reflecting schema: %w", err)
	}
	// Take a local copy so the &desc pointer is safe.
	desc := r.Description
	opt := openapi3.WithName("default", &openapi3.Response{
		Description: &desc,
		Content: map[string]*openapi3.MediaType{
			"application/json": {Schema: schemaRef},
		},
	})
	return openapi3.NewResponses(opt), nil
}

func buildParameters(params []sdk.ApiParameter) []*openapi3.ParameterRef {
	refs := make([]*openapi3.ParameterRef, 0, len(params))
	for _, p := range params {
		refs = append(refs, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:        p.Name,
				In:          p.In,
				Description: p.Description,
				Required:    p.Required,
				Schema:      openapi3.NewStringSchema().NewRef(),
			},
		})
	}
	return refs
}

// rewritePathParams converts Fiber-style ":id" segments to OpenAPI
// "{id}" segments.
func rewritePathParams(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) > 1 && p[0] == ':' {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// ─── HTML rendering ─────────────────────────────────────────────

// htmlData is the template context used to render index.html.tmpl.
type htmlData struct {
	Title      string
	DocName    string
	JSName     string // sanitized DocName for use in JS identifiers
	LogoURL    string
	MadeBy     string
	SpecURL    string // URL to the OpenAPI YAML (relative to mount root)
	Theme      string
	DarkMode   bool
	ServerJSON string // JSON array of server hosts for client-side filtering
}

// RenderHTML renders the Scalar docs page. specPath is the URL
// from which the client should fetch the OpenAPI spec — typically
// "openapi.yaml" (served by the adapter alongside the HTML).
func (d *Docs) RenderHTML(specPath string) ([]byte, error) {
	data := htmlData{
		Title:      d.cfg.Title,
		DocName:    d.cfg.DocName,
		JSName:     sanitizeJSIdent(d.cfg.DocName),
		LogoURL:    d.cfg.LogoURL,
		MadeBy:     d.cfg.MadeBy,
		SpecURL:    specPath,
		Theme:      d.cfg.Theme,
		DarkMode:   d.cfg.DarkMode,
		ServerJSON: buildServerHostsJSON(d.cfg.Servers),
	}

	var buf bytes.Buffer
	if err := d.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("mtnuu: rendering HTML: %w", err)
	}
	return buf.Bytes(), nil
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// sanitizeJSIdent produces a safe JS identifier fragment from an
// arbitrary DocName. "My Cool API" → "MyCoolAPI".
// Guarantees a non-empty, alphanumeric result; falls back to "Mtnuu"
// if the input reduces to nothing.
func sanitizeJSIdent(s string) string {
	out := nonAlnum.ReplaceAllString(s, "")
	if out == "" {
		return "Mtnuu"
	}
	// Ensure it doesn't start with a digit.
	if out[0] >= '0' && out[0] <= '9' {
		out = "X" + out
	}
	return out
}

// buildServerHostsJSON emits a JSON array literal of hostnames for
// embedding directly into a <script> tag. We strip the scheme so the
// client-side matcher can stay a simple substring check.
func buildServerHostsJSON(servers []config.Server) string {
	if len(servers) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(servers))
	for _, s := range servers {
		host := stripScheme(s.URL)
		if host == "" {
			continue
		}
		parts = append(parts, jsonString(host))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func stripScheme(u string) string {
	u = strings.TrimSpace(u)
	for _, p := range []string{"https://", "http://", "//"} {
		if strings.HasPrefix(u, p) {
			return strings.TrimSuffix(u[len(p):], "/")
		}
	}
	return strings.TrimSuffix(u, "/")
}

// jsonString does a minimal JSON-safe quote. We avoid pulling encoding/json
// just for this — the inputs are short, ASCII host strings.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}