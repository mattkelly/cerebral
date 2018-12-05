package nodeutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	//"k8s.io/apimachinery/pkg/labels"
)

func TestGetNodesLabelSelector(t *testing.T) {
	s := GetNodesLabelSelector(nil)
	reqs, _ := s.Requirements()
	assert.Len(t, reqs, 0, "nil map has no requirements")

	s = GetNodesLabelSelector(map[string]string{
		"key1": "val1",
		"key2": "val2",
	})
	reqs, _ = s.Requirements()
	assert.Len(t, reqs, 2, "number of keys = number of requirements out")
}
