package queue

import (
	"time"
)

type TimeQueue struct {
	vals []*time.Time
	head int
	tail int
	size int
}

func NewTimeQueue(capacity int) *TimeQueue {
	return &TimeQueue{
		vals: make([]*time.Time, capacity),
	}
}

func (q *TimeQueue) Push(val *time.Time) {
	if q.size == len(q.vals) {
		q.resize()
	}

	q.vals[q.tail] = val
	q.tail = (q.tail + 1) % len(q.vals)
	q.size++
}

func (q *TimeQueue) Head() (*time.Time, error) {
	if q.size == 0 {
		return nil, ErrEmptyQueue
	}

	return q.vals[q.head], nil
}

func (q *TimeQueue) Pop() (*time.Time, error) {
	if q.size == 0 {
		return nil, ErrEmptyQueue
	}

	val := q.vals[q.head]
	q.head = (q.head + 1) % len(q.vals)
	q.size--

	return val, nil
}

func (q *TimeQueue) Size() int {
	return q.size
}

// we only increase size (do not decrease)
func (q *TimeQueue) resize() {
	newsize := q.size * 2

	vals := make([]*time.Time, newsize)
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
