package ghttp

import "context"

// Limiter controls outbound request rate.
type Limiter interface {
	Wait(ctx context.Context) error
}
