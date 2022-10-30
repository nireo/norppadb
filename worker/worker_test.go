package worker_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/nireo/norppadb/worker"
)

func TestQueue(t *testing.T) {
	var wg sync.WaitGroup
	for i := 10; i < 100000; i *= 10 {
		wg.Add(i)
		q := worker.NewQueue(i)
		for j := 0; j < i; j++ {
			go func(w int) {
				q <- func() {
					dur := time.Duration(rand.Intn(10))
					time.Sleep(dur * time.Millisecond)
					wg.Done()
				}
			}(j)
		}
		wg.Wait()
		close(q)
	}
}
