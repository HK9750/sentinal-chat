package commands

import "context"

type Proxy interface {
	Authorize(ctx context.Context, cmd Command) error
}
