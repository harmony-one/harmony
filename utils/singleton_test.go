package utils

import (
	"sync"
	"testing"
	"time"
)

var NumThreads = 20

func TestSingleton(t *testing.T) {
	si := GetUniqueValidatorIDInstance()
	var wg sync.WaitGroup

	t.Log("unique ID provided by singleton instance")

	for i := 0; i < NumThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Logf("id:%v\n", si.GetUniqueID())
			time.Sleep(time.Millisecond)

		}()
	}
	wg.Wait()

	t.Log("non-unique ID")
	n := 100
	for i := 0; i < NumThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Log("num:", n)
			n++
			time.Sleep(time.Millisecond)
		}()
	}

	wg.Wait()
}
