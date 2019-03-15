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
	// This alert channel has N writers - the pollers that are created here.
	// Since its lifetime is matched to this function, we'll just let it be
	// garbage collected if this function exits.
	alertCh := make(chan alert, len(m.pollers))

	// This error channel is used essentially as a return value for any requests
	// to the scale manager. Similar to the alert channel, we'll just let it
	// be garbage collected.
	errCh := make(chan error)

	// This stop channel tells the pollers to stop if this poll manager is
	// shutting down for any reason. It must be closed if this function exits.
	pollerStopCh := make(chan struct{})

	var wg sync.WaitGroup
	for _, p := range m.pollers {
		wg.Add(1)
		go p.run(&wg, alertCh, pollerStopCh)
	}

	// Make sure that when this poll manager dies, all of its pollers are properly
	// cleaned up.
	defer func() {
		close(pollerStopCh)

		// Wait for all pollers to shut down
		wg.Wait()

		log.Infof("Poll manager for AutoscalingGroup %s shut down success", m.asgName)
	}()

	for {
		select {
		case alert := <-alertCh:
			if alert.err != nil {
				return errors.Wrapf(alert.err, "polling metrics for ASP")
			}

			asp := m.asps[alert.aspName]

			if alert.direction == scaleDirectionUp {
				m.recorder.Event(asp, corev1.EventTypeNormal, events.ScaleUpAlerted,
					fmt.Sprintf("Alert triggered to scale up by %.2f (%s)",
						alert.adjustmentValue, alert.adjustmentType.String()))
			} else {
				m.recorder.Event(asp, corev1.EventTypeNormal, events.ScaleDownAlerted,
					fmt.Sprintf("Alert triggered to scale down by %.2f (%s)",
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
				return errors.Wrap(err, "requesting scale manager to scale")
			}

		case <-m.stopCh:
			log.Infof("Poll manager for AutoscalingGroup %s shutting down", m.asgName)
			return nil
		}
	}
}
