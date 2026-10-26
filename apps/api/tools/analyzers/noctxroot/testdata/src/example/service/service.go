package service

import (
	"context"
	stdctx "context"
)

func Do() {
	_ = context.Background() // want `context.Background\(\) is only allowed in cmd/ and _test.go`
	_ = stdctx.TODO()        // want `context.TODO\(\) is only allowed`
}

func Good(ctx context.Context) { _ = context.WithoutCancel(ctx) }
