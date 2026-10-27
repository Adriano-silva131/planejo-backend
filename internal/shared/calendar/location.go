package calendar

import (
	"context"
	"time"
)

type locationKey struct{}

// WithLocation stores the user's time zone in ctx.
func WithLocation(ctx context.Context, loc *time.Location) context.Context {
	return context.WithValue(ctx, locationKey{}, loc)
}

// LocationFrom returns the user's time zone stored in ctx, or UTC.
func LocationFrom(ctx context.Context) *time.Location {
	if loc, ok := ctx.Value(locationKey{}).(*time.Location); ok {
		return loc
	}
	return time.UTC
}
