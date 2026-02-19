package gorawrsquirrel

import (
	"math/rand"

	"github.com/Keksclan/goRawrSquirrel/cache"
	"github.com/Keksclan/goRawrSquirrel/internal/core"
	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/security"
	"github.com/Keksclan/goRawrSquirrel/tracing"
)

// config holds the internal configuration assembled via functional options.
type config struct {
	middlewares core.MiddlewareBuilder
	resolver    *policy.Resolver
	ipBlocker   *security.IPBlocker
	cache       cache.Cache
	l1          *cache.L1
	l2          *cache.L2
	tracing     *tracing.TracingConfig
	funMode     bool
	funRand     rand.Source
	funMessages []string
}
