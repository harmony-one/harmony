package stagedsync

import (
	"time"

	"github.com/Workiva/go-datastructures/queue"
)

// downloadTaskQueue is wrapper around Queue with item to be SyncBlockTask
type downloadTaskQueue struct {
	q *queue.Queue
}

func (queue downloadTaskQueue) poll(num int64, timeOut time.Duration) (syncBlockTasks, error) {
	items, err := queue.q.Poll(num, timeOut)
	if err != nil {
		return nil, err
	}
	tasks := make(syncBlockTasks, 0, len(items))
	for _, item := range items {
		task := item.(SyncBlockTask)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (queue downloadTaskQueue) put(tasks syncBlockTasks) error {
	for _, task := range tasks {
		if err := queue.q.Put(task); err != nil {
			return err
		}
	}
	return nil
}

func (queue downloadTaskQueue) empty() bool {
	return queue.q.Empty()
}
