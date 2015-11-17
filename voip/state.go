package voip

import (
	"strconv"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

const (
	ubufsize = 100
)

type State struct {
	// control parameters
	nfconts map[string]*MContainer
	uchan   chan string
	mger    CManager

	// config parameters
	step_length     int64
	period_length   int64
	reference       int64
	cpu_usage_table string
	rx_table        string
	tx_table        string
}

func NewState(config *goconfig.ConfigFile) (*State, error) {
	step_length, err := config.Int64("VOIP.CONTROL", "step_length")
	if err != nil {
		return nil, err
	}

	period_length, err := config.Int64("VOIP.CONTROL", "period_length")
	if err != nil {
		return nil, err
	}

	reference, err := config.Int64("VOIP.CONTROL", "reference")
	if err != nil {
		return nil, err
	}

	cpu_table, err := config.GetValue("VOIP.CONTROL", "cpu_table")
	if err != nil {
		return nil, err
	}

	rx_table, err := config.GetValue("VOIP.CONTROL", "rx_table")
	if err != nil {
		return nil, err
	}

	tx_table, err := config.GetValue("VOIP.CONTROL", "tx_table")
	if err != nil {
		return nil, err
	}

	var mger CManager
	mtype, err := config.GetValue("VOIP.MANAGER", "type")
	if err != nil {
		return nil, err
	}
	switch mtype {
	case "docker":
		mger, err = NewDockerCManager(config)
	case "stack":
		mger, err = NewStackCManager(config)
	default:
		err = ErrUnknownManager
	}
	if err != nil {
		return nil, err
	}

	return &State{
		nfconts: make(map[string]*MContainer),
		uchan:   make(chan string, ubufsize),
		mger:    mger,

		step_length:     step_length,
		period_length:   period_length,
		reference:       reference,
		cpu_usage_table: cpu_table,
		rx_table:        rx_table,
		tx_table:        tx_table,
	}, nil
}

func (s *State) Destroy() {
	close(s.uchan)
	s.mger.Destroy()
}

func (s *State) Trigger() {
	for _, cont := range s.nfconts {
		shares := cont.Trigger()
		s.mger.SetShares(cont.id, shares)
	}
}

func (s *State) Update(points models.Points) {
	select {
	case nf := <-s.uchan:
		shares, _ := strconv.ParseInt(<-s.uchan, 10, 64)
		s.nfconts[nf] = NewMContainer(nf, s.step_length, s.period_length, shares, s.reference)
	default:
	}

	if len(s.nfconts) == 0 {
		return
	}

	for _, point := range points {
		cont, ok := s.nfconts[point.Tags()["container_name"]]
		if !ok {
			return
		}

		switch point.Name() {
		case s.cpu_usage_table:
			cont.AddPoint(CPU_TABLE, point)
		case s.rx_table:
			cont.AddPoint(RX_TABLE, point)
		case s.tx_table:
			cont.AddPoint(TX_TABLE, point)
		default:
		}
	}

	s.Trigger()
}
