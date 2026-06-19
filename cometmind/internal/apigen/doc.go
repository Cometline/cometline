// Package apigen holds OpenAPI-generated REST wire types for the CometMind local API.
//
//go:generate go tool oapi-codegen -generate types -package apigen -o types.gen.go ../../openapi.yaml
package apigen
