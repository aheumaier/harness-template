// Package fxleak violates go-layers: platform must not import a domain.
package fxleak

import "github.com/aheumaier/harness-template/apps/api/internal/fx/model"

// Zero returns a domain value from platform code.
func Zero() model.Item { return model.Item{} }
