// Package service violates go-domain-isolation.
package service

import "github.com/aheumaier/harness-template/apps/api/internal/fx/store"

// Count reads another domain's store directly.
func Count() int { return store.Count() }
