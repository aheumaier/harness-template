// Package service violates go-layers by importing the api layer.
package service

import "github.com/aheumaier/harness-template/apps/api/internal/fx/api"

// Version leaks the api layer into the service.
func Version() string { return api.Version }
