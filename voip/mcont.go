package voip

import (
	"log"
	"time"

	"github.com/influxdb/influxdb/models"
)

const (
	RX_TABLE = iota
	TX_TABLE
	CPU_TABLE
)

type MContainer struct {
	id      string
	inflow  *TimeData
	outflow *TimeData
	cpuload *TimeData

	// algorithm vars
	ploadr  float64
	prxr    float64
	ptxr    float64
	ibytes  int64
	tibytes int64
	csum    int64

	// control vars
	share int64
	ref   int64
}

func NewMContainer(id string, step, wl, share, ref int64) *MContainer {
	curtime := time.Now()

	return &MContainer{
		id:      id,
		inflow:  NewTimeData(step, wl, curtime),
		outflow: NewTimeData(step, wl, curtime),
		cpuload: NewTimeData(step, wl, curtime),
		share:   share,
		ref:     ref,
	}
}

func (m *MContainer) AddPoint(table int, point models.Point) {
	val, ok := point.Fields()["value"].(int64)
	if !ok {
		log.Println("[_WARN] unknown data type!")
		return
	}

	switch table {
	case RX_TABLE:
		m.inflow.AddPoint(point.Time(), val)
	case TX_TABLE:
		m.outflow.AddPoint(point.Time(), val)
	case CPU_TABLE:
		m.cpuload.AddPoint(point.Time(), val)
	default:
		panic("not reachable!")
	}
}

func (m *MContainer) Trigger() int64 {
	for {
		_, rxr, ok1 := m.inflow.Next()
		tx, txr, ok2 := m.outflow.Next()
		_, cpr, ok3 := m.inflow.Next()
		if !ok1 || !ok2 || !ok3 {
			break
		}

		// init, only once
		if m.ibytes == 0 {
			m.ibytes = tx
			m.tibytes = tx
		}

		// we have three points rx, tx, cp synchronized within <step>
		duration := m.inflow.AfterD()
		ninetyp := float64(m.share) / 1024 * 90
		switch {
		// 1 interval is over, we look at only inflow for consistency
		case duration > 0:
			dprime := m.csum / m.share / duration
			delta := (tx - m.ibytes) / duration
			m.share -= (m.ref - delta) / dprime

			// reinitialize
			m.csum = 0
			m.ibytes = tx
		case m.ploadr > ninetyp && m.ploadr < ninetyp:
			m.csum += (tx - m.tibytes) * int64(m.prxr/(m.prxr-m.ptxr))
			m.tibytes = tx
		case m.ploadr < ninetyp && m.ploadr > ninetyp:
			m.tibytes = tx
		case m.ploadr > ninetyp && m.ploadr > ninetyp:
		case m.ploadr < ninetyp && cpr < ninetyp:
		default:
		}

		m.ploadr = cpr
		m.prxr = rxr
		m.ptxr = txr
	}

	return m.share
}
