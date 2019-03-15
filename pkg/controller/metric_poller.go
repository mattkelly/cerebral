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

var nowFunc = time.Now

func (a *alertState) start() {
	a.startTime = nowFunc()
	a.active = true
}

func (a *alertState) shouldFire(samplePeriod time.Duration) bool {
	return a.active && nowFunc().Sub(a.startTime) >= samplePeriod
}

func newMetricPoller(asp *v1alpha1.AutoscalingPolicy, nodeSelector map[string]string) metricPoller {
	return metricPoller{
		asp:          asp,
		nodeSelector: nodeSelector,
	}
}

func sendAlert(alertCh chan<- alert, msg alert) {
	select {
	case alertCh <- msg:
	default:
		log.Debugf("Alert channel is full. Discarding alert %v", msg)
	}
}

func (p metricPoller) run(wg *sync.WaitGroup, alertCh chan<- alert, stopCh <-chan struct{}) {
	defer wg.Done()

	pollInterval := time.Duration(p.asp.Spec.PollInterval) * time.Second
	samplePeriod := time.Duration(p.asp.Spec.SamplePeriod) * time.Second
	policyName := p.asp.ObjectMeta.Name
	backendName := p.asp.Spec.MetricsBackend
	metric := p.asp.Spec.Metric
	metricConfig := p.asp.Spec.MetricConfiguration

	upAlert := &alertState{}
	downAlert := &alertState{}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			backend, err := metrics.Registry().Get(backendName)
			if err != nil {
				err = errors.Wrapf(err, "metrics backend %q specified by policy %q is unavailable", backendName, policyName)
				sendAlert(alertCh, alert{err: err})
				return
			}

			val, err := backend.GetValue(metric, metricConfig, p.nodeSelector)
			if err != nil {
				err = errors.Wrapf(err, "getting metric %q for policy %q", metric, policyName)
				sendAlert(alertCh, alert{err: err})
				return
			}

			log.Debugf("Poller for ASP %q got value %f", policyName, val)

			// Scale up alerts
			upConfig := p.asp.Spec.ScalingPolicy.ScaleUp
			if policyConfigurationShouldFireAlert(upConfig, upAlert, samplePeriod, val) {
				p.fireAlert(alertCh, upConfig, scaleDirectionUp)
			}

			// Scale down alerts
			downConfig := p.asp.Spec.ScalingPolicy.ScaleDown
			if policyConfigurationShouldFireAlert(downConfig, downAlert, samplePeriod, val) {
				p.fireAlert(alertCh, downConfig, scaleDirectionDown)
			}

		case <-stopCh:
			log.Debugf("Poller for ASP %s shutting down", p.asp.ObjectMeta.Name)
			return
		}
	}
}

func (p *metricPoller) fireAlert(alertCh chan<- alert, policy *v1alpha1.ScalingPolicyConfiguration, dir scaleDirection) {
	// Thanks to CRD validation, we can assume that this is valid
	adjustmentType, _ := adjustmentTypeFromString(policy.AdjustmentType)
	sendAlert(alertCh, alert{
		aspName:         p.asp.ObjectMeta.Name,
		direction:       dir,
		adjustmentType:  adjustmentType,
		adjustmentValue: policy.AdjustmentValue,
	})
}

func policyConfigurationShouldFireAlert(policy *v1alpha1.ScalingPolicyConfiguration,
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
