package containership

import (
	"encoding/json"
	"errors"
	"os"
)

type cloudConfig struct {
	Address         string
	TokenEnvVarName string
	OrganizationID  string
	ClusterID       string
}

func (c *cloudConfig) defaultAndValidate(configuration map[string]string) error {
	// Round trip the config through JSON parser to populate our struct
	j, _ := json.Marshal(configuration)
	json.Unmarshal(j, c)

	if err := c.defaultAndValidateAddress(); err != nil {
		return err
	}

	if err := c.defaultAndValidateTokenEnvVarName(); err != nil {
		return err
	}

	if err := c.defaultAndValidateOrganizationID(); err != nil {
		return err
	}

	if err := c.defaultAndValidateClusterID(); err != nil {
		return err
	}

	return nil
}

func (c *cloudConfig) defaultAndValidateAddress() error {
	if c.Address == "" {
		c.Address = "https://provision.containership.io"
	}

	return nil
}

func (c *cloudConfig) defaultAndValidateTokenEnvVarName() error {
	if c.TokenEnvVarName == "" {
		return errors.New("tokenEnvVarName must be provided")
	}

	token := os.Getenv(c.TokenEnvVarName)
	if token == "" {
		return errors.New("unable to get Containership Cloud API cluster token")
	}

	return nil
}

func (c *cloudConfig) defaultAndValidateOrganizationID() error {
	if c.OrganizationID == "" {
		return errors.New("organizationID must be provided")
	}

	return nil
}

func (c *cloudConfig) defaultAndValidateClusterID() error {
	if c.ClusterID == "" {
		return errors.New("clusterID must be provided")
	}

	return nil
}
