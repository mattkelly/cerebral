package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var valid = []string{">", "<", ">=", "<=", "==", "!="}

const invalid = "hi"
const empty = ""

type evalTest struct {
	lhs float64
	op  ComparisonOperator
	rhs float64

	expected bool
}

var evalTests = []evalTest{
	{lhs: 1, op: GreaterThan, rhs: 0, expected: true},
	{lhs: 1, op: GreaterThan, rhs: 1, expected: false},
	{lhs: 0, op: GreaterThan, rhs: 1, expected: false},

	{lhs: 1, op: LessThan, rhs: 0, expected: false},
	{lhs: 1, op: LessThan, rhs: 1, expected: false},
	{lhs: 0, op: LessThan, rhs: 1, expected: true},

	{lhs: 1, op: GreaterThanEqual, rhs: 0, expected: true},
	{lhs: 1, op: GreaterThanEqual, rhs: 1, expected: true},
	{lhs: 0, op: GreaterThanEqual, rhs: 1, expected: false},

	{lhs: 1, op: LessThanEqual, rhs: 0, expected: false},
	{lhs: 1, op: LessThanEqual, rhs: 1, expected: true},
	{lhs: 0, op: LessThanEqual, rhs: 1, expected: true},

	{lhs: 1, op: Equal, rhs: 0, expected: false},
	{lhs: 1, op: Equal, rhs: 1, expected: true},

	{lhs: 1, op: NotEqual, rhs: 0, expected: true},
	{lhs: 1, op: NotEqual, rhs: 1, expected: false},

	{lhs: 1, op: -1, rhs: 1, expected: false},
}

func TestFromString(t *testing.T) {
	for _, s := range valid {
		_, err := FromString(s)
		assert.NoError(t, err, "valid operator")
	}

	_, err := FromString(invalid)
	assert.Error(t, err, "invalid operator")

	_, err = FromString(empty)
	assert.Error(t, err, "empty operator")
}

func TestEvaluate(t *testing.T) {
	for _, test := range evalTests {
		result := test.op.Evaluate(test.lhs, test.rhs)
		assert.Equal(t, test.expected, result, "%#v", test)
	}
}
