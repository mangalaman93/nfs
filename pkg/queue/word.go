package queue

type WordQueue struct {
	vals []int64
	head int
	tail int
	size int
}

func NewWordQueue(capacity int) *WordQueue {
	return &WordQueue{
		vals: make([]int64, capacity),
	}
}

func (q *WordQueue) Push(val int64) {
	if q.size == len(q.vals) {
		q.resize()
	}

	q.vals[q.tail] = val
	q.tail = (q.tail + 1) % len(q.vals)
	q.size++
}

func (q *WordQueue) Head() (int64, error) {
	if q.size == 0 {
		return 0, ErrEmptyQueue
	}

	return q.vals[q.head], nil
}

func (q *WordQueue) Pop() (int64, error) {
	if q.size == 0 {
		return 0, ErrEmptyQueue
	}

	val := q.vals[q.head]
	q.head = (q.head + 1) % len(q.vals)
	q.size--

	return val, nil
}

func (q *WordQueue) Size() int {
	return q.size
}

// we only increase size (do not decrease)
func (q *WordQueue) resize() {
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
