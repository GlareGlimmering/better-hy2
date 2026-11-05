package util

import (
	"strconv"
	"strings"
)

func CompareVersion(version1, version2 string) int {
	v1 := strings.Split(version1, ".")
	v2 := strings.Split(version2, ".")

	maxLen := len(v1)
	if len(v2) > maxLen {
		maxLen = len(v2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int

		if i < len(v1) {
			n1, _ = strconv.Atoi(v1[i])
		}

		if i < len(v2) {
			n2, _ = strconv.Atoi(v2[i])
		}

		if n1 > n2 {
			return 1
		} else if n1 < n2 {
			return -1
		}
	}

	return 0
}
