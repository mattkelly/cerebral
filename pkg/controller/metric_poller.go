package controller

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/containership/cluster-manager/pkg/log"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/operator"
)

type metricPoller struct {
	asp          *v1alpha1.AutoscalingPolicy
	nodeSelector map[string]string
}

type alertState struct {
	active    bool
	startTime time.Time
}

func newMetricPoller(asp *v1alpha1.AutoscalingPolicy, nodeSelector map[string]string) metricPoller {
	return metricPoller{
		asp:          asp,
		nodeSelector: nodeSelector,
	}
}

var nowFunc = time.Now

func (p metricPoller) run(wg *sync.WaitGroup, alertCh chan<- alert, stopCh <-chan struct{}) {
	defer wg.Done()

	pollInterval := time.Duration(p.asp.Spec.PollInterval) * time.Second
	samplePeriod := time.Duration(p.asp.Spec.SamplePeriod) * time.Second
	policyName := p.asp.ObjectMeta.Name
	backendName := p.asp.Spec.MetricsBackend
	metric := p.asp.Spec.Metric
	metricConfig := p.asp.Spec.MetricConfiguration

	var upAlert = &alertState{}
	var downAlert = &alertState{}

	ticker := time.NewTicker(pollInterval)
	for {
		select {
		case <-ticker.C:
			backend, err := metrics.Registry().Get(backendName)
			if err != nil {
				err = errors.Wrapf(err, "metrics backend %q specified by policy %q is unavailable", backendName, policyName)
				alertCh <- alert{err: err}
				return
			}

			val, err := backend.GetValue(metric, metricConfig, p.nodeSelector)
			if err != nil {
				err = errors.Wrapf(err, "getting metric %q for policy %q", metric, policyName)
				alertCh <- alert{err: err}
				return
			}

			log.Debugf("Poller for ASP %q got value %f", policyName, val)

			// Check for scale up alerts
			shouldAlertUp := evaluatePolicyConfiguration(
				p.asp.Spec.ScalingPolicy.ScaleUp, upAlert, samplePeriod, val)
			if shouldAlertUp {
				alertCh <- alert{
					aspName:   p.asp.ObjectMeta.Name,
					direction: "up",
				}
			}

			shouldAlertDown := evaluatePolicyConfiguration(
				p.asp.Spec.ScalingPolicy.ScaleDown, downAlert, samplePeriod, val)
			if shouldAlertDown {
				alertCh <- alert{
					aspName:   p.asp.ObjectMeta.Name,
					direction: "down",
				}
			}

		case <-stopCh:
			log.Debugf("Poller for ASP %s shutting down", p.asp.ObjectMeta.Name)
			return
		}
	}
}

func evaluatePolicyConfiguration(policy *v1alpha1.ScalingPolicyConfiguration,
	alert *alertState, samplePeriod time.Duration, val float64) bool {
	if policy == nil {
		// Nothing to do
		return false
	}

	// Assume the operator is correct thanks to OpenAPI validation on the CR
	op, _ := operator.FromString(policy.ComparisonOperator)
	if !op.Evaluate(val, policy.Threshold) {
		// We're not alerting, so nothing to do
		return false
	}

	if !alert.active {
		// We just started alerting, so update the alert state accordingly
		alert.start()
		return false
	}

	if alert.shouldFire(samplePeriod) {
		// We've been in an active alert state for the entire sample period,
		// so fire an alert
		alert.active = false
		return true
	}

	return false
}

func (a *alertState) start() {
	a.startTime = nowFunc()
	a.active = true
}

func (a *alertState) shouldFire(samplePeriod time.Duration) bool {
	return a.active && nowFunc().Sub(a.startTime) >= samplePeriod
}
