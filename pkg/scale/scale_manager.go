package scale

import (
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
)

type Request struct {
	Type                 string // TODO up/down
	Count                int
	IgnoreCoolDown       bool
	AutoscalingGroupName string
}

/*
const (
	ScaleUp ScaleRequest = iota
	ScaleDown
)

func (s ScaleRequest) String() string {
	switch s {
	case ScaleUp:
		return "ScaleUp"
	case ScaleDown:
		return "ScaleDown"
	}

	return "Unknown"
}

func RequestScale(req ScaleRequest) error {

}
*/

type Manager struct {
	clientset cerebral.Interface
}

func NewManager(clientset cerebral.Interface) *Manager {
	return &Manager{
		clientset: clientset,
	}
}

func (m Manager) ProcessRequests(requests <-chan Request) {

}
