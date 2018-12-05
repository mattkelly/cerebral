package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

func setTime(seconds int64) {
	nowFunc = func() time.Time {
		return time.Unix(seconds, 0)
	}
}

func resetTime() {
	nowFunc = time.Now
}

func TestNewMetricPoller(t *testing.T) {
	p := newMetricPoller(&v1alpha1.AutoscalingPolicy{}, map[string]string{})
	assert.NotNil(t, p, "never nil")
}

func TestEvaluatePolicyConfiguration(t *testing.T) {
	fired := evaluatePolicyConfiguration(nil, &alertState{}, time.Second, 0)
	assert.False(t, fired, "nil config is a noop")

	alert := &alertState{active: false}
	gteConfig := &v1alpha1.ScalingPolicyConfiguration{
		Threshold:          75,
		ComparisonOperator: ">=",
	}

	fired = evaluatePolicyConfiguration(gteConfig, alert, 5*time.Second, 10)
	assert.False(t, fired, "have not breached threshold")

	alert = &alertState{active: true, startTime: time.Unix(0, 0)}
	setTime(2)
	fired = evaluatePolicyConfiguration(gteConfig, alert, 5*time.Second, 80)
	assert.False(t, fired, "breached threshold but not long enough")

	alert = &alertState{active: false}
	setTime(2)
	fired = evaluatePolicyConfiguration(gteConfig, alert, 5*time.Second, 80)
	assert.False(t, fired, "breached threshold but not active")

	alert = &alertState{active: true, startTime: time.Unix(0, 0)}
	setTime(10)
	fired = evaluatePolicyConfiguration(gteConfig, alert, 5*time.Second, 80)
	assert.True(t, fired, "breached threshold for long enough")

	alert = &alertState{active: false, startTime: time.Unix(0, 0)}
	setTime(2)
	fired = evaluatePolicyConfiguration(gteConfig, alert, 5*time.Second, 10)
	assert.False(t, fired, "breached threshold but not long enough")

	resetTime()
}

func TestAlertShouldFire(t *testing.T) {
	inactive := &alertState{active: false}
	fired := inactive.shouldFire(0)
	assert.False(t, fired, "inactive alert shouldn't fire")

	alert := &alertState{active: true, startTime: time.Unix(0, 0)}
	setTime(2)
	fired = alert.shouldFire(5 * time.Second)
	assert.False(t, fired, "alert active but has not reached sample period")

	alert = &alertState{active: true, startTime: time.Unix(0, 0)}
	setTime(5)
	fired = alert.shouldFire(5 * time.Second)
	assert.True(t, fired, "alert active and equal to sample period")

	alert = &alertState{active: true, startTime: time.Unix(0, 0)}
	setTime(10)
	fired = alert.shouldFire(5 * time.Second)
	assert.True(t, fired, "alert active and exceeded sample period")

	resetTime()
}
