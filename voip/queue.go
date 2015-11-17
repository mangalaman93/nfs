package voip

import (
	"errors"
)

var (
	ErrEmptyQueue = errors.New("Queue is empty")
)

// see reference https://github.com/ErikDubbelboer/ringqueue
type RQueue struct {
	vals []int64
	head int
	tail int
	size int
}

func NewRQueue(capacity int) *RQueue {
	return &RQueue{
		vals: make([]int64, capacity),
	}
}

func (q *RQueue) Push(val int64) {
	if q.size == len(q.vals) {
		q.resize()
	}

	q.vals[q.tail] = val
	q.tail = (q.tail + 1) % len(q.vals)
	q.size++
}

func (q *RQueue) Pop() (int64, error) {
	if q.size == 0 {
		return 0, ErrEmptyQueue
	}

	val := q.vals[q.head]
	q.head = (q.head + 1) % len(q.vals)
	q.size--

	return val, nil
}

func (q *RQueue) Size() int {
	return q.size
}

// we only increase size (do not decrease)
func (q *RQueue) resize() {
	newsize := q.size * 2

	vals := make([]int64, newsize)
	if q.head < q.tail {
		copy(vals, q.vals[q.head:q.tail])
	} else {
		copy(vals, q.vals[q.head:])
		copy(vals[len(q.vals)-q.head:], q.vals[:q.tail])
	}

	q.tail = q.size % newsize
	q.head = 0
	q.vals = vals
}
