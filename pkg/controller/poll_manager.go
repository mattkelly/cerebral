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

	scaleRequestCh chan<- ScaleRequest

	stopCh chan struct{}
}

type alert struct {
	aspName         string
	direction       scaleDirection
	adjustmentType  adjustmentType
	adjustmentValue float64

	err error
}

func newPollManager(asgName string, asps []*v1alpha1.AutoscalingPolicy, nodeSelector map[string]string,
	scaleRequestCh chan<- ScaleRequest, stopCh chan struct{}) pollManager {
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

	errCh := make(chan error)

	for {
		select {
		case alert := <-alertCh:
			if alert.err != nil {
				return errors.Wrapf(alert.err, "polling metrics for ASP")
			}

			m.scaleRequestCh <- ScaleRequest{
				asgName:         m.asgName,
				direction:       alert.direction,
				adjustmentType:  alert.adjustmentType,
				adjustmentValue: alert.adjustmentValue,
				errCh:           errCh,
			}

			err := <-errCh
			if err != nil {
				// If a scale request fails, just return an error so the relevant ASG can be re-enqueued
				return errors.Wrap(err, "requesting scale manager to scale")
			}

		case <-m.stopCh:
			log.Infof("Poll manager for AutoscalingGroup %s shutdown requested", m.asgName)
			wg.Wait()
			log.Infof("Poll manager for AutoscalingGroup %s shutdown success", m.asgName)
			return nil
		}
	}
}
