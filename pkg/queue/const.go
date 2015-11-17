package queue

import (
	"errors"
)

var (
	ErrEmptyQueue = errors.New("Queue is empty")
)

// see reference https://github.com/ErikDubbelboer/ringqueue
