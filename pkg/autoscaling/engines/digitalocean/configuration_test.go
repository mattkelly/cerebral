package digitalocean

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// ConfigKeyClusterID is the configuration key that is used for getting the cluster ID
	ConfigKeyClusterID = "clusterID"
	// ConfigKeyTokenEnvVarName is the name of the configuration key that is env var
	// which is used to get the DigitalOcean API key
	ConfigKeyTokenEnvVarName = "tokenEnvVarName"
	// ConfigKeyNodePoolLabelKey is the configuration key used for getting the
	// node pool ID out of the node selector being used in an ASG
	ConfigKeyNodePoolLabelKey = "nodePoolLabelKey"
)

func TestValidateConfiguration(t *testing.T) {
	configuration := map[string]string{
		ConfigKeyTokenEnvVarName: "TOKEN_ENV_VAR",
		ConfigKeyClusterID:       "cluster-uuid",
	}

	os.Setenv(configuration[ConfigKeyTokenEnvVarName], "token")
	defer os.Unsetenv(configuration[ConfigKeyTokenEnvVarName])

	c := cloudConfig{}
	err := c.defaultAndValidate(configuration)
	assert.NoError(t, err)
	assert.Equal(t, configuration[ConfigKeyTokenEnvVarName], c.TokenEnvVarName)
	assert.Equal(t, configuration[ConfigKeyClusterID], c.ClusterID)

	for key := range configuration {
		existingValue := configuration[key]
		delete(configuration, key)
		c = cloudConfig{}
		err = c.defaultAndValidate(configuration)
		assert.Error(t, err, fmt.Sprintf("Testing that an error is returned when client configuration is missing %q", key))
		configuration[key] = existingValue
	}
}
