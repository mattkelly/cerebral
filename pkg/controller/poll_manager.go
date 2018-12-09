package controller

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/record"

	"github.com/containership/cluster-manager/pkg/log"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/events"
)

type pollManager struct {
	asgName string

	// Keys are ASP name
	asps    map[string]*v1alpha1.AutoscalingPolicy
	pollers map[string]metricPoller

	recorder record.EventRecorder

	scaleRequestCh chan<- ScaleRequest
	stopCh         chan struct{}
}

type alert struct {
	aspName         string
	direction       scaleDirection
	adjustmentType  adjustmentType
	adjustmentValue float64

	err error
}

func newPollManager(asgName string, asps map[string]*v1alpha1.AutoscalingPolicy, nodeSelector map[string]string,
	recorder record.EventRecorder,
	scaleRequestCh chan<- ScaleRequest, stopCh chan struct{}) pollManager {
	mgr := pollManager{
		asgName:        asgName,
		asps:           asps,
		pollers:        make(map[string]metricPoller),
		recorder:       recorder,
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

			asp := m.asps[alert.aspName]

			if alert.direction == scaleDirectionUp {
				m.recorder.Event(asp, corev1.EventTypeNormal, events.ScaleUpAlerted,
					fmt.Sprintf("alert triggered to scale up by %.2f (%s)",
						alert.adjustmentValue, alert.adjustmentType.String()))
			} else {
				m.recorder.Event(asp, corev1.EventTypeNormal, events.ScaleDownAlerted,
					fmt.Sprintf("alert triggered to scale down by %.2f (%s)",
						alert.adjustmentValue, alert.adjustmentType.String()))
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
			// All pollers share the manager's stop chan so no need to
			// stop them explicitly
			wg.Wait()
			log.Infof("Poll manager for AutoscalingGroup %s shutdown success", m.asgName)
			return nil
		}
	}
}
