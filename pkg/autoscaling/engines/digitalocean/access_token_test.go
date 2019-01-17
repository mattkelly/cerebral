package digitalocean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	token = "access-token"
)

func TestToken(t *testing.T) {
	ts := tokenSource{
		AccessToken: token,
	}

	ot, err := ts.Token()
	assert.NoError(t, err)
	assert.Equal(t, token, ot.AccessToken)
}
