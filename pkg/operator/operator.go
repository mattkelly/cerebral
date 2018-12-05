package operator

import (
	"github.com/pkg/errors"
)

type ComparisonOperator int

const (
	GreaterThan ComparisonOperator = iota
	LessThan
	GreaterThanEqual
	LessThanEqual
	Equal
	NotEqual
)

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
