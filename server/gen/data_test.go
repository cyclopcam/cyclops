package gen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteFirst(t *testing.T) {
	a := []int{1, 2, 3}
	b := DeleteFirst(a, -1)
	require.Equal(t, a, b)

	a = []int{1, 2, 3}
	b = DeleteFirst(a, 1)
	require.ElementsMatch(t, []int{2, 3}, b)

	a = []int{1, 2, 3}
	b = DeleteFirst(a, 2)
	require.ElementsMatch(t, []int{1, 3}, b)

	a = []int{1, 2, 3}
	b = DeleteFirst(a, 3)
	require.ElementsMatch(t, []int{1, 2}, b)

	a = []int{1, 2}
	b = DeleteFirst(a, 1)
	require.ElementsMatch(t, []int{2}, b)

	a = []int{1}
	b = DeleteFirst(a, 1)
	require.ElementsMatch(t, []int{}, b)
}
