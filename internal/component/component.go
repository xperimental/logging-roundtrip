package component

import (
	"context"
	"sync"
)

type Component interface {
	Start(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error)
}
