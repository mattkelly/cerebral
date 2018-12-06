package controller

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/containership/cluster-manager/pkg/log"

	v1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/operator"
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
				return errors.Wrapf(alert.err, "polling ASP")
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

func (m *pollManager) addPoller(p metricPoller) {
}

type metricPoller struct {
	asp          *v1alpha1.AutoscalingPolicy
	nodeSelector map[string]string
}

func newMetricPoller(asp *v1alpha1.AutoscalingPolicy, nodeSelector map[string]string) metricPoller {
	return metricPoller{
		asp:          asp,
		nodeSelector: nodeSelector,
	}
}

type alertState struct {
	active bool
	start  time.Time
}

func (p metricPoller) run(wg *sync.WaitGroup, alertCh chan<- alert, stopCh <-chan struct{}) {
	defer wg.Done()

	ticker := time.NewTicker(time.Duration(p.asp.Spec.PollInterval) * time.Second)

	log.Debugf("Poller for ASP %s is ready", p.asp.ObjectMeta.Name)

	var upAlert /*, downAlert */ alertState

	for {
		select {
		case <-ticker.C:
			policyName := p.asp.ObjectMeta.Name
			backend, err := metrics.Registry().Get(p.asp.Spec.MetricsBackend)
			if err != nil {
				err = errors.Wrapf(err, "metrics backend %q specified by policy %q is unavailable", p.asp.Spec.MetricsBackend, policyName)
				alertCh <- alert{err: err}
				return
			}

			val, err := backend.GetValue(p.asp.Spec.Metric, p.asp.Spec.MetricConfiguration, p.nodeSelector)
			if err != nil {
				err = errors.Wrapf(err, "getting metric %q for policy %q", p.asp.Spec.Metric, policyName)
				alertCh <- alert{err: err}
				return
			}

			log.Debugf("Poller for ASP %q got value %f", policyName, val)

			// Check for scale up alerts
			// TODO refactor to easily check scale down alerts as well
			up := p.asp.Spec.ScalingPolicy.ScaleUp
			if up != nil {
				op, err := operator.FromString(up.ComparisonOperator)
				if err != nil {
					alertCh <- alert{err: err}
					return
				}

				if !op.Evaluate(val, up.Threshold) {
					// Nothing to do
					continue
				}

				if upAlert.active {
					if time.Now().Sub(upAlert.start) >= time.Duration(p.asp.Spec.SamplePeriod)*time.Second {
						alertCh <- alert{
							aspName:   p.asp.ObjectMeta.Name,
							direction: "up",
						}
						upAlert.active = false
					}
				} else {
					upAlert.start = time.Now()
					upAlert.active = true
				}
			}

		case <-stopCh:
			log.Debugf("Poller for ASP %s shutting down", p.asp.ObjectMeta.Name)
			return
		}
	}
}
