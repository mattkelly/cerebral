package operator

import (
	"github.com/pkg/errors"
)

// ComparisonOperator is a mathematical comparison operator for comparing floats
type ComparisonOperator int

const (
	// GreaterThan is the > operator
	GreaterThan ComparisonOperator = iota
	// LessThan is the < operator
	LessThan
	// GreaterThanEqual is the >= operator
	GreaterThanEqual
	// LessThanEqual is the <= operator
	LessThanEqual
	// Equal is the == operator
	Equal
	// NotEqual is the != operator
	NotEqual
)

// String returns a string representation of the given operator
func (o ComparisonOperator) String() string {
	switch o {
	case GreaterThan:
		return ">"
	case LessThan:
		return "<"
	case GreaterThanEqual:
		return ">="
	case LessThanEqual:
		return "<="
	case Equal:
		return "=="
	case NotEqual:
		return "!="
	}

	return "unknown"
}

// FromString converts a string to a ComparisonOperator type or returns an error
func FromString(s string) (ComparisonOperator, error) {
	switch s {
	case GreaterThan.String():
		return GreaterThan, nil
	case LessThan.String():
		return LessThan, nil
	case GreaterThanEqual.String():
		return GreaterThanEqual, nil
	case LessThanEqual.String():
		return LessThanEqual, nil
	case Equal.String():
		return Equal, nil
	case NotEqual.String():
		return NotEqual, nil
	}

	return 0, errors.Errorf("invalid operator %q", s)
}

// Evaluate evaluates the result of the expression: lhs (op) rhs
func (o ComparisonOperator) Evaluate(lhs float64, rhs float64) bool {
	switch o {
	case GreaterThan:
		return lhs > rhs
	case LessThan:
		return lhs < rhs
	case GreaterThanEqual:
		return lhs >= rhs
	case LessThanEqual:
		return lhs <= rhs
	case Equal:
		return lhs == rhs
	case NotEqual:
		return lhs != rhs
	}

	return false
}
