package voip

import (
	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

type State struct {
	period_num      int64
	period_length   int64
	cpu_usage_table string
	rx_table        string
	tx_table        string
}

func NewState(config *goconfig.ConfigFile) (*State, error) {
	period_length, err := config.Int64("VOIP.CONTROL", "period_length")
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

	return &State{
		period_num:      0,
		period_length:   period_length,
		cpu_usage_table: cpu_table,
		rx_table:        rx_table,
		tx_table:        tx_table,
	}, nil
}

func (s *State) Update(points models.Points) {
}
