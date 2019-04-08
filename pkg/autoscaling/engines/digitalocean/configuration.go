package digitalocean

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
)

type cloudConfig struct {
	TokenEnvVarName string
	ClusterID       string
}

func (c *cloudConfig) defaultAndValidate(configuration map[string]string) error {
	// Round trip the config through JSON parser to populate our struct
	j, _ := json.Marshal(configuration)
	json.Unmarshal(j, c)

	if c.ClusterID == "" {
		return errors.Errorf("clusterID must be provided")
	}

	if c.TokenEnvVarName == "" || os.Getenv(c.TokenEnvVarName) == "" {
		return errors.New("tokenEnvVarName must be provided, and reference a valid env var")
	}

	return nil
}
