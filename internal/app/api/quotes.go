// internal/app/api/quotes.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Quotes struct {
	svc *service.Service
	ctx func() context.Context
}

func NewQuotes(svc *service.Service, ctx func() context.Context) *Quotes {
	return &Quotes{svc: svc, ctx: ctx}
}

func (q *Quotes) Watch(consumer string, tokens []uint32) (service.WatchResult, error) {
	return q.svc.WatchQuotes(q.ctx(), consumer, tokens)
}
