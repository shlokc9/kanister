package kopia

import (
	"fmt"
	"strings"
)

// The helpers here are used to print kopia upload progress in bytes
// Duplicated here since they are a part of an internal package

var (
	base10UnitPrefixes = []string{"", "K", "M", "G", "T"}
	base2UnitPrefixes  = []string{"", "Ki", "Mi", "Gi", "Ti"}
)

func niceNumber(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", f), "0"), ".")
}

func toDecimalUnitString(f, thousand float64, prefixes []string, suffix string) string {
	for i := range prefixes {
		if f < 0.9*thousand {
			return fmt.Sprintf("%v %v%v", niceNumber(f), prefixes[i], suffix)
		}

		f /= thousand
	}

	return fmt.Sprintf("%v %v%v", niceNumber(f), prefixes[len(prefixes)-1], suffix)
}

// BytesStringBase10 formats the given value as bytes with the appropriate base-10 suffix (KB, MB, GB, ...)
func BytesStringBase10(b int64) string {
	return toDecimalUnitString(float64(b), 1000, base10UnitPrefixes, "B")
}
