package utils

import (
	"fmt"
	"strings"
	"time"
)

var monthNameMapping = map[string]string{
	"январь":   "January",
	"февраль":  "February",
	"март":     "March",
	"апрель":   "April",
	"май":      "May",
	"июнь":     "June",
	"июль":     "July",
	"август":   "August",
	"сентябрь": "September",
	"октябрь":  "October",
	"ноябрь":   "November",
	"декабрь":  "December",
}

// ParseMonth returns a time.Time object from a Russian month string and year.
func ParseMonth(monthStr, yearStr string) (*time.Time, error) {
	month := strings.ToLower(monthStr)
	// Attempt to parse using Russian month names directly
	parsedTime, err := time.Parse("2 January 2006", fmt.Sprintf("1 %s %s", monthNameMapping[month], yearStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse month and year: %v", err)
	}
	return &parsedTime, nil
}
