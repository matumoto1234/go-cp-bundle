package addutil

import "cmp"

func Add[T cmp.Ordered](a, b T) T {
	return a + b
}
