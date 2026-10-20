// Package reffectorskill exposes the canonical embedded agent skill.
package reffectorskill

import _ "embed"

// Content is the exact SKILL.md content installed by reffector skill install.
//
//go:embed SKILL.md
var Content []byte
