package gorawrsquirrel

import "github.com/Keksclan/goRawrSquirrel/internal/core"

// config holds the internal configuration assembled via functional options.
type config struct {
	middlewares core.MiddlewareBuilder
}
