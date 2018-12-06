package controller

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/operator"
)

type pollManager struct {
	asg *cerebralv1alpha1.AutoscalingGroup

	// Key is ASP name
	pollers map[string]metricPoller

	stopCh chan struct{}
}

type alert struct {
	aspName   string
	direction string
	err       error
}

func newPollManager(asg *cerebralv1alpha1.AutoscalingGroup, stopCh chan struct{}) pollManager {
	return pollManager{
		asg:     asg,
		stopCh:  stopCh,
		pollers: make(map[string]metricPoller),
	}
}

func (m *pollManager) run() error {
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

			log.Infof("alert received from ASP %s", alert.aspName)

		case <-m.stopCh:
			log.Infof("Poll manager for AutoscalingGroup %s shutdown requested", m.asg.ObjectMeta.Name)
			wg.Wait()
			log.Infof("Poll manager for AutoscalingGroup %s shutdown success", m.asg.ObjectMeta.Name)
			return nil
		}
	}
}

func (m *pollManager) addPoller(p metricPoller) {
	m.pollers[p.asp.ObjectMeta.Name] = p
}

type metricPoller struct {
	asp   cerebralv1alpha1.AutoscalingPolicy
	nodes []*corev1.Node
}

func newMetricPoller(asp cerebralv1alpha1.AutoscalingPolicy, nodes []*corev1.Node) metricPoller {
	return metricPoller{
		asp:   asp,
		nodes: nodes,
	}
}

type alertState struct {
	active bool
	start  time.Time
}

func (p metricPoller) run(wg *sync.WaitGroup, alertCh chan<- alert, stopCh <-chan struct{}) error {
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
				alertCh <- alert{err: err}
				return errors.Errorf("backend %q specified by ASP %q is unavailable",
					p.asp.Spec.MetricsBackend, policyName)
			}

			val, err := backend.GetValue(p.asp.Spec.Metric, p.asp.Spec.MetricConfiguration, p.nodes)
			if err != nil {
				alertCh <- alert{err: err}
				return errors.Errorf("error getting metric %q for policy %q: %s",
					p.asp.Spec.Metric, policyName, err)
			}

			log.Debugf("Poller for ASP %q got value %f", policyName, val)

			// Check for scale up alerts
			// TODO refactor to easily check scale down alerts as well
			up := p.asp.Spec.ScalingPolicy.ScaleUp
			if up != nil {
				op, err := operator.FromString(up.ComparisonOperator)
				if err != nil {
					alertCh <- alert{err: err}
					return err
				}

				if !op.Evaluate(val, up.Threshold) {
					// Nothing to do
					continue
				}

				log.Info("Policy alerting!")
				if upAlert.active {
					if time.Now().Sub(upAlert.start) >= time.Duration(p.asp.Spec.SamplePeriod)*time.Second {
						log.Info("******* Up policy should scale now!")
						upAlert.active = false
					}
				} else {
					upAlert.start = time.Now()
					upAlert.active = true
				}
			}

		case <-stopCh:
			log.Debugf("Poller for ASP %s shutting down", p.asp.ObjectMeta.Name)
			return nil
		}
	}
}
