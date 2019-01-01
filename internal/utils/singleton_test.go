package utils

import (
	"sync"
	"testing"
	"time"
)

var NumThreads = 20

func TestSingleton(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < NumThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)

		}()
	}
	wg.Wait()

	n := 100
	for i := 0; i < NumThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n++
			time.Sleep(time.Millisecond)
		}()
	}

	wg.Wait()
}
