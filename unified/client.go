package unified

import "context"

type Client interface {
	Request(ctx context.Context, req Request) (<-chan Event, error)
}
