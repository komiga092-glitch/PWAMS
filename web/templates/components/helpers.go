package components

import (
	"fmt"
	"strconv"
)

func itoa(i int) string {
	return strconv.Itoa(i)
}

func formatDecimal(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
