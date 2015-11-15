package voip

import (
	"time"
)

type TimeData struct {
	vals              []int64
	tstamps           []time.Time
	step, duration    int
	index, iter_index int
}

// all data in ms
func NewTimeData(step int, duration int) *TimeData {
	length := duration / step * 2

	return &TimeData{
		tstamps:    make([]time.Time, 0, length),
		vals:       make([]int64, 0, length),
		step:       step,
		duration:   duration,
		iter_index: 0,
	}
}

// TODO
func (td *TimeData) AddPoint(ts time.Time, val int64) {
	td.tstamps[td.index] = ts
	td.vals[td.index] = val
	td.index += 1
}
