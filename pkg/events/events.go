package events

const (
	// ScaleUpAlerted event is created when an AutoscalingPolicy scale up alert occurs
	ScaleUpAlerted = "ScaleUpAlerted"
	// ScaleDownAlerted event is created when an AutoscalingPolicy scale down alert occurs
	ScaleDownAlerted = "ScaleDownAlerted"

	// ScaledUp event is created when an AutoscalingGroup is scaled up
	ScaledUp = "ScaledUp"
	// ScaledDown event is created when an AutoscalingGroup is scaled down
	ScaledDown = "ScaledDown"

	// ScaleIgnored event is created when a scale event is ignored
	ScaleIgnored = "ScaleIgnored"

	// ScaleError event is created when a scale event errors
	ScaleError = "ScaleError"
)
