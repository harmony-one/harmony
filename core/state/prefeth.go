package state

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"log"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type prefetchJob struct {
	accountAddr []byte
	account     *Account
	start, end  []byte
}

// Prefetch If redis is empty, the hit rate will be too low and the synchronization block speed will be slow
// this function will parallel load the latest block statedb to redis
func (s *DB) Prefetch(parallel int) {
	wg := sync.WaitGroup{}

	jobChan := make(chan *prefetchJob, 10000)
	waitWorker := int64(parallel)

	// start parallel workers
	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			for job := range jobChan {
				atomic.AddInt64(&waitWorker, -1)
				s.prefetchWorker(job, jobChan)
				atomic.AddInt64(&waitWorker, 1)
			}
		}()

		wg.Add(1)
	}

	// add first jobs
	for i := 0; i < 255; i++ {
		start := []byte{byte(i)}
		end := []byte{byte(i + 1)}
		if i == parallel-1 {
			end = nil
		} else if i == 0 {
			start = nil
		}

		jobChan <- &prefetchJob{
			start: start,
			end:   end,
		}
	}

	// wait all worker done
	start := time.Now()
	log.Println("Prefetch start")
	var sleepCount int
	for sleepCount < 3 {
		time.Sleep(time.Second)
		waitWorkerCount := atomic.LoadInt64(&waitWorker)
		if waitWorkerCount >= int64(parallel) {
			sleepCount++
		} else {
			sleepCount = 0
		}
	}

	close(jobChan)
	wg.Wait()
	end := time.Now()
	log.Println("Prefetch end, use", end.Sub(start))
}

// prefetchWorker used to process one job
func (s *DB) prefetchWorker(job *prefetchJob, jobs chan *prefetchJob) {
	if job.account == nil {
		// scan one account
		nodeIterator := s.trie.NodeIterator(job.start)
		it := trie.NewIterator(nodeIterator)

		for it.Next() {
			if job.end != nil && bytes.Compare(it.Key, job.end) >= 0 {
				return
			}

			// build account data from main trie tree
			var data Account
			if err := rlp.DecodeBytes(it.Value, &data); err != nil {
				panic(err)
			}
			addrBytes := s.trie.GetKey(it.Key)
			addr := common.BytesToAddress(addrBytes)
			obj := newObject(s, addr, data)
			if data.CodeHash != nil {
				obj.Code(s.db)
			}

			// build account trie tree
			storageIt := trie.NewIterator(obj.getTrie(s.db).NodeIterator(nil))
			storageJob := &prefetchJob{
				accountAddr: addrBytes,
				account:     &data,
			}

			// fetch data
			s.prefetchAccountStorage(jobs, storageJob, storageIt)
		}
	} else {
		// scan main trie tree
		obj := newObject(s, common.BytesToAddress(job.accountAddr), *job.account)
		storageIt := trie.NewIterator(obj.getTrie(s.db).NodeIterator(job.start))

		// fetch data
		s.prefetchAccountStorage(jobs, job, storageIt)
	}
}

// bytesMiddle get the binary middle value of a and b
func (s *DB) bytesMiddle(a, b []byte) []byte {
	if a == nil && b == nil {
		return []byte{128}
	}

	if len(a) > len(b) {
		tmp := make([]byte, len(a))
		if b == nil {
			for i, _ := range tmp {
				tmp[i] = 255
			}
		}
		copy(tmp, b)
		b = tmp
	} else if len(a) < len(b) {
		tmp := make([]byte, len(b))
		if a == nil {
			for i, _ := range tmp {
				tmp[i] = 0
			}
		}
		copy(tmp, a)
		a = tmp
	}

	aI := big.NewInt(0).SetBytes(a)
	bI := big.NewInt(0).SetBytes(b)

	aI.Add(aI, bI)
	aI.Div(aI, big.NewInt(2))
	return aI.Bytes()
}

// prefetchAccountStorage used for fetch account storage
func (s *DB) prefetchAccountStorage(jobs chan *prefetchJob, job *prefetchJob, it *trie.Iterator) {
	start := time.Now()
	count := 0

	for it.Next() {
		if job.end != nil && bytes.Compare(it.Key, job.end) >= 0 {
			return
		}

		count++

		// if fetch one account job used more than 15s, then split the job
		if count%10000 == 0 && time.Now().Sub(start) > 15*time.Second {
			middle := s.bytesMiddle(it.Key, job.end)
			startJob := &prefetchJob{
				accountAddr: job.accountAddr,
				account:     job.account,
				start:       it.Key,
				end:         middle,
			}
			endJob := &prefetchJob{
				accountAddr: job.accountAddr,
				account:     job.account,
				start:       middle,
				end:         job.end,
			}

			select {
			case jobs <- endJob:
				select {
				case jobs <- startJob:
					return
				default:
					job.end = startJob.end
					start = time.Now()
				}
			default:
				log.Println("job full, continue")
			}
		}
	}
}
