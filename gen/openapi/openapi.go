// Package openapi embeds the OpenAPI documents generated from proto/ by `buf generate`
// (written to v2/). This file is hand-written; buf only replaces the v2/ directory.
package openapi

import _ "embed"

// Registry is the OpenAPI v2 document for spotter.v1.RegistryService.
//
//go:embed v2/spotter/v1/registry.swagger.json
var Registry []byte
