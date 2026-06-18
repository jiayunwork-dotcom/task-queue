package ratelimit

import (
	"container/list"
	"sync"
	"time"

	"github.com/google/uuid"
)

type waitingTask struct {
	taskID    uuid.UUID
	enqueuedAt time.Time
}

type WaitQueue struct {
	queues      map[string]*list.List
	releaseCh     chan uuid.UUID
	mu           sync.Mutex
}

func NewWaitQueue() *WaitQueue {
	return &WaitQueue{
		queues:  make(map[string]*list.List),
		releaseCh: make(chan uuid.UUID, 10000),
	}
}

func (wq *WaitQueue) Enqueue(taskType string, taskID uuid.UUID, now time.Time) {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	if _, ok := wq.queues[taskType]; !ok {
		wq.queues[taskType] = list.New()
	}
	wq.queues[taskType].PushBack(&waitingTask{
		taskID:    taskID,
		enqueuedAt: now,
	})
}

func (wq *WaitQueue) Dequeue(taskType string, count int) []uuid.UUID {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	queue, ok := wq.queues[taskType]
	if !ok || queue.Len() == 0 {
		return nil
	}

	actualCount := count
	if queue.Len() < count {
		actualCount = queue.Len()
	}

	result := make([]uuid.UUID, 0, actualCount)
	for i := 0; i < actualCount; i++ {
		elem := queue.Front()
		if elem == nil {
			break
		}
		wt := elem.Value.(*waitingTask)
		result = append(result, wt.taskID)
		queue.Remove(elem)

		select {
		case wq.releaseCh <- wt.taskID:
		default:
		}
	}

	if queue.Len() == 0 {
		delete(wq.queues, taskType)
	}

	return result
}

func (wq *WaitQueue) ReleaseAll(taskType string) {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	queue, ok := wq.queues[taskType]
	if !ok {
		return
	}

	for queue.Len() > 0 {
		elem := queue.Front()
		if elem == nil {
			break
		}
		wt := elem.Value.(*waitingTask)
		queue.Remove(elem)

		select {
		case wq.releaseCh <- wt.taskID:
		default:
		}
	}

	delete(wq.queues, taskType)
}

func (wq *WaitQueue) GetTaskTypes() []string {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	types := make([]string, 0, len(wq.queues))
	for t := range wq.queues {
		types = append(types, t)
	}
	return types
}

func (wq *WaitQueue) GetWaitCount(taskType string) int {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	if queue, ok := wq.queues[taskType]; ok {
		return queue.Len()
	}
	return 0
}

func (wq *WaitQueue) GetAllWaitCounts() map[string]int {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	result := make(map[string]int, len(wq.queues))
	for t, q := range wq.queues {
		result[t] = q.Len()
	}
	return result
}

func (wq *WaitQueue) PeekCount(taskType string) int {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	if queue, ok := wq.queues[taskType]; ok {
		return queue.Len()
	}
	return 0
}

func (wq *WaitQueue) Peek(taskType string, count int) []uuid.UUID {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	queue, ok := wq.queues[taskType]
	if !ok || queue.Len() == 0 {
		return nil
	}

	actualCount := count
	if queue.Len() < count {
		actualCount = queue.Len()
	}

	result := make([]uuid.UUID, 0, actualCount)
	elem := queue.Front()
	for i := 0; i < actualCount && elem != nil; i++ {
		wt := elem.Value.(*waitingTask)
		result = append(result, wt.taskID)
		elem = elem.Next()
	}
	return result
}

func (wq *WaitQueue) ReleaseChan() <-chan uuid.UUID {
	return wq.releaseCh
}
