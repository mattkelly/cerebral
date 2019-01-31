package containership

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAndValidate(t *testing.T) {
	c := cloudConfig{}
	err := c.defaultAndValidate(nil)
	assert.Error(t, err, "nil config provided is invalid")

	c = cloudConfig{}
	err = c.defaultAndValidate(map[string]string{})
	assert.Error(t, err, "empty config provided is invalid")

	configuration := map[string]string{
		"address":         "https://provision-test.containership.io",
		"tokenEnvVarName": "TOKEN_ENV_VAR",
		"organizationID":  "organization-uuid",
		"clusterID":       "cluster-uuid",
	}

	os.Setenv(configuration["tokenEnvVarName"], "token")

	c = cloudConfig{}
	err = c.defaultAndValidate(configuration)
	assert.NoError(t, err, "good config")
	assert.Equal(t, "https://provision-test.containership.io", c.Address, "address not defaulted if provided")

	os.Unsetenv(configuration["tokenEnvVarName"])

	configurationWithoutAddress := map[string]string{
		"tokenEnvVarName": "TOKEN_ENV_VAR",
		"organizationID":  "organization-uuid",
		"clusterID":       "cluster-uuid",
	}

	os.Setenv(configurationWithoutAddress["tokenEnvVarName"], "token")

	c = cloudConfig{}
	err = c.defaultAndValidate(configurationWithoutAddress)
	assert.NoError(t, err, "good config")
	assert.Equal(t, "https://provision.containership.io", c.Address, "address defaulted if not provided")

	for key := range configurationWithoutAddress {
		c = cloudConfig{}
		existingValue := configurationWithoutAddress[key]
		delete(configurationWithoutAddress, key)
		err = c.defaultAndValidate(configurationWithoutAddress)
		assert.Error(t, err, fmt.Sprintf("Testing that an error is returned when client configuration is missing %q", key))
		configurationWithoutAddress[key] = existingValue
	}
	os.Unsetenv(configurationWithoutAddress["tokenEnvVarName"])
}
