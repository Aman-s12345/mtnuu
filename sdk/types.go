// Package sdk exposes the public types consumers of mtnuu use
// when registering their API routes for OpenAPI generation.
//
// These types are intentionally kept in a separate package so that
// feature packages (e.g. your routes/*) only need to import sdk —
// not the full docs service — when declaring their OpenAPI metadata.
package sdk

// ApiWrapper is the metadata a consumer attaches to each route.
// It mirrors the shape of an OpenAPI operation and is converted
// into one by service.Docs.GenerateOpenAPI.
type ApiWrapper struct {
	// Path is the route path. Fiber-style :param segments are
	// rewritten to OpenAPI {param} form during generation.
	Path string `json:"path"`

	// Method is the HTTP verb (GET, POST, PUT, DELETE, PATCH).
	Method string `json:"method"`

	// Name is shown as the operation summary in Scalar.
	Name string `json:"name"`

	// Description is the longer-form body text for the operation.
	Description string `json:"description"`

	// Tags groups operations in the Scalar sidebar.
	Tags []string `json:"tags,omitempty"`

	// RequestBody describes the JSON body the route expects.
	RequestBody *ApiRequestBody `json:"requestBody,omitempty"`

	// Response describes the JSON body the route returns.
	Response *ApiResponse `json:"response,omitempty"`

	// Parameters describes query, path, or header parameters.
	Parameters []ApiParameter `json:"parameters,omitempty"`

	// UnAuthenticated, when true, omits the BearerAuth requirement
	// from the generated operation.
	UnAuthenticated bool `json:"unauthenticated,omitempty"`
}

// ApiParameter describes a single query/path/header parameter.
type ApiParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // "query" | "path" | "header"
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ApiRequestBody wraps a sample Go value whose struct shape is
// reflected into a JSON schema by the OpenAPI generator.
type ApiRequestBody struct {
	Description string      `json:"description,omitempty"`
	Content     interface{} `json:"content,omitempty"`
}

// ApiResponse wraps a sample Go value whose struct shape is
// reflected into a JSON schema by the OpenAPI generator.
type ApiResponse struct {
	Description string      `json:"description,omitempty"`
	Content     interface{} `json:"content,omitempty"`
}