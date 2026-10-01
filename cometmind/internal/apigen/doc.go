// Package apigen holds OpenAPI-generated REST types and the Gin strict server
// for the CometMind local API.
//
//go:generate go tool oapi-codegen -config types.cfg.yaml ../../openapi.yaml
//go:generate go tool oapi-codegen -config server.cfg.yaml ../../openapi.yaml
package apigen
