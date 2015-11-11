package nfsmain

import (
	"github.com/influxdb/influxdb/models"
)

type State struct {
}

func (s *State) Trigger() {
}

func (s *State) Update(points models.Points) {
	for _, point := range points {
		switch point.Name() {
		}
	}
}
