// Package schema embeds the DSL JSON Schema so `twinmodel schema` can print it and
// editors can consume it for autocomplete/validation.
package schema

import _ "embed"

//go:embed twinmodel.schema.json
var JSON string
