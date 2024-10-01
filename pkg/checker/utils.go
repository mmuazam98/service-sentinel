package checker

import (
	"regexp"
)

func StripANSI(s string) string {
	re := regexp.MustCompile("\x1B\\[[;\\d]*m")
	return re.ReplaceAllString(s, "")
}
