package controller

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/containership/cluster-manager/pkg/log"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

type pollManager struct {
	asgName string
	asps    []*v1alpha1.AutoscalingPolicy

	// Key is ASP name
	pollers map[string]metricPoller

	// TODO should be of type chan ScaleRequest
	scaleRequestCh chan struct{}

	stopCh chan struct{}
}

type alert struct {
	aspName   string
	direction string
	err       error
}

func newPollManager(asgName string, asps []*v1alpha1.AutoscalingPolicy, nodeSelector map[string]string, scaleRequestCh, stopCh chan struct{}) pollManager {
	mgr := pollManager{
		asgName:        asgName,
		asps:           asps,
		pollers:        make(map[string]metricPoller),
		scaleRequestCh: scaleRequestCh,
		stopCh:         stopCh,
	}

	for _, asp := range asps {
		p := newMetricPoller(asp, nodeSelector)
		mgr.pollers[asp.ObjectMeta.Name] = p
	}

	return mgr
}

func (m pollManager) run() error {
	var wg sync.WaitGroup
	alertCh := make(chan alert)

	for _, p := range m.pollers {
		wg.Add(1)
		go p.run(&wg, alertCh, m.stopCh)
	}

	for {
		select {
		case alert := <-alertCh:
			if alert.err != nil {
				return errors.Wrapf(alert.err, "polling metrics for ASP")
			}

			// TODO forward scale request
			log.Infof("Alert received from ASP %s in direction %s", alert.aspName, alert.direction)

		case <-m.stopCh:
			log.Infof("Poll manager for AutoscalingGroup %s shutdown requested", m.asgName)
			wg.Wait()
			log.Infof("Poll manager for AutoscalingGroup %s shutdown success", m.asgName)
			return nil
		}
	}
}
