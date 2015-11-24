package voip

import (
	"log"
	"math"
	"time"

	"github.com/influxdb/influxdb/models"
)

const (
	RX_TABLE = iota
	TX_TABLE
	CPU_TABLE
)

type MContainer struct {
	node *Node

	// data
	inflow  *TimeData
	outflow *TimeData
	cpuload *TimeData

	// control vars
	shares int64
	ref    int64

	// algorithm vars
	ploadr  float64
	prxr    float64
	ptxr    float64
	ibytes  int64
	tibytes int64
	csum    int64
}

func NewMContainer(node *Node, step, wl, shares, ref int64) *MContainer {
	curtime := time.Now()

	return &MContainer{
		node:    node,
		inflow:  NewTimeData(step, wl, curtime),
		outflow: NewTimeData(step, wl, curtime),
		cpuload: NewTimeData(step, wl, curtime),
		shares:  shares,
		ref:     ref,
	}
}

func (m *MContainer) AddPoint(table int, point models.Point) {
	fval, ok := point.Fields()["value"].(float64)
	if !ok {
		log.Println("[WARN] unknown data type!")
		return
	}

	val := int64(fval)
	switch table {
	case RX_TABLE:
		m.inflow.AddPoint(point.Time(), val)
	case TX_TABLE:
		m.outflow.AddPoint(point.Time(), val)
	case CPU_TABLE:
		m.cpuload.AddPoint(point.Time(), val)
	default:
		panic("NOT REACHABLE")
	}
}

func (m *MContainer) Trigger() int64 {
	for {
		_, rxr, ok1 := m.inflow.Next()
		tx, txr, ok2 := m.outflow.Next()
		_, cpr, ok3 := m.cpuload.Next()
		cpr /= 10000000
		if !ok1 || !ok2 || !ok3 {
			break
		}

		if m.ibytes == 0 {
			m.ibytes = tx
			m.tibytes = tx
		}

		// we have three points rx, tx, cp synchronized within <step>
		ninetyp := float64(m.shares) * 90 / 1024

		switch {
		case m.ploadr > ninetyp && m.ploadr < ninetyp:
			m.csum += (tx - m.tibytes) * int64(m.prxr/(m.prxr+m.ptxr))
			m.tibytes = tx
		case m.ploadr < ninetyp && m.ploadr > ninetyp:
			m.tibytes = tx
		case m.ploadr > ninetyp && m.ploadr > ninetyp:
		case m.ploadr < ninetyp && cpr < ninetyp:
		}

		// 1 interval is over, we look at only inflow for consistency
		duration := float64(m.inflow.AfterD())
		if duration > 0 {
			dprime := float64(m.csum) / float64(m.shares) / duration
			if math.Abs(dprime) > 0.01 {
				delta := float64(tx-m.ibytes) / duration
				m.shares -= int64((float64(m.ref) - delta) / dprime)
			}

			m.csum = 0
			m.ibytes = tx
		}

		m.ploadr = cpr
		m.prxr = rxr
		m.ptxr = txr
	}

	return m.shares
}
