package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/kubernetestest"
)

var fakeEngineConfiguration = map[string]string{
	"address":         "https://provision-test.containership.io",
	"tokenEnvVarName": "TOKEN_ENV_VAR",
	"organizationID":  "organization-uuid",
	"clusterID":       "cluster-uuid",
}

var fakeContainershipASE = &cerebralv1alpha1.AutoscalingEngine{
	ObjectMeta: metav1.ObjectMeta{
		Name: "containership-autoscaling-engine",
	},
	Spec: cerebralv1alpha1.AutoscalingEngineSpec{
		Type:          "containership",
		Configuration: fakeEngineConfiguration,
	},
}

var fakeInvalidASE = &cerebralv1alpha1.AutoscalingEngine{
	ObjectMeta: metav1.ObjectMeta{
		Name: "invalid-autoscaling-engine",
	},
	Spec: cerebralv1alpha1.AutoscalingEngineSpec{
		Type:          "invalid",
		Configuration: fakeEngineConfiguration,
	},
}

func TestInstantiateEngine(t *testing.T) {
	os.Setenv(fakeEngineConfiguration["tokenEnvVarName"], "token")
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{})

	c, err := instantiateEngine(fakeContainershipASE)
	assert.NoError(t, err, "Test that engine instantiation does not error")
	assert.NotNil(t, c, "Test that engine is instantiated")

	c, err = instantiateEngine(fakeInvalidASE)
	assert.Error(t, err, "Test that engine instantiation errors for invalid type")

	os.Unsetenv(fakeEngineConfiguration["tokenEnvVarName"])
}
