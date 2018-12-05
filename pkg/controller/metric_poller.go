package controller

import (
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/operator"
)

type pollManager struct {
	group *cerebralv1alpha1.AutoscalingGroup

	// map ASP name --> poller
	pollers map[string]metricPoller
}

type metricPoller struct {
	policy *cerebralv1alpha1.AutoscalingPolicy
	nodes  []*corev1.Node

	requestScale chan struct{}
	stopCh       chan struct{}

	upAlert   alertState
	downAlert alertState
}

type alertState struct {
	active bool
	start  time.Time
}

func (p *metricPoller) run() error {
	ticker := time.NewTicker(time.Duration(p.policy.Spec.PollInterval) * time.Second)

	for {
		select {
		case <-ticker.C:
			policyName := p.policy.ObjectMeta.Name
			backend, err := metrics.Registry().Get(p.policy.Spec.MetricsBackend)
			if err != nil {
				return errors.Errorf("backend %q specified by policy %q is unavailable",
					p.policy.Spec.MetricsBackend, policyName)
			}

			val, err := backend.GetValue(p.policy.Spec.Metric, p.policy.Spec.MetricConfiguration, p.nodes)
			if err != nil {
				return errors.Errorf("error getting metric %q for policy %q: %s",
					p.policy.Spec.Metric, policyName, err)
			}

			log.Debugf("Poller for policy %q got val %f", policyName, val)

			// Check for scale up alerts
			up := p.policy.Spec.ScalingPolicy.ScaleUp
			if up != nil {
				op, err := operator.FromString(up.ComparisonOperator)
				if err != nil {
					return err
				}

				if op.Evaluate(val, up.Threshold) {
					log.Info("Up policy alerting!")
					alert := &p.upAlert
					if alert.active {
						if time.Now().Sub(alert.start) >= time.Duration(p.policy.Spec.SamplePeriod)*time.Second {
							log.Info("******* Up policy should scale now!")
							alert.active = false
						}
					} else {
						alert.start = time.Now()
						alert.active = true
					}
				}
			}

			// Check for scale down alerts
			down := p.policy.Spec.ScalingPolicy.ScaleDown
			if down != nil {
				op, err := operator.FromString(down.ComparisonOperator)
				if err != nil {
					return err
				}

				if op.Evaluate(val, down.Threshold) {
					log.Info("Down policy alerting!")
					alert := &p.downAlert
					if alert.active {
						if time.Now().Sub(alert.start) >= time.Duration(p.policy.Spec.SamplePeriod)*time.Second {
							log.Info("******* Down policy should scale now!")
							alert.active = false
						}
					} else {
						alert.start = time.Now()
						alert.active = true
					}
				}
			}

		case <-p.stopCh:
			log.Debugf("Poller for policy %s shutting down", p.policy.ObjectMeta.Name)
			return nil
		}
	}
}
