package service

import (
	barapi "example/internal/bar/api"     // want `domain "foo" imports api of domain "bar"`
	barsvc "example/internal/bar/service" // allowed: cross-domain via service
	barstore "example/internal/bar/store" // want `domain "foo" imports store of domain "bar"`
	"example/internal/foo/store"          // allowed: own store
)

var _ = barapi.A + barsvc.V + barstore.S + store.S
