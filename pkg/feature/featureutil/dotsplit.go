package featureutil

import (
	"fmt"
	"strings"
)

func SmartDotSplit(s string) ([]string, error) {
	if strings.HasSuffix(s, ".") {
		return nil, fmt.Errorf("cannot end with `.`")
	}
	if strings.HasPrefix(s, ".") {
		return nil, fmt.Errorf("cannot start with `.`")
	}

	str := ""
	var ret []string
	for i, ch := range s {
		switch ch {
		case '.':
			if len(str) == 0 || i == 0 {
				return nil, fmt.Errorf("invalid `.` on position %v", i)
			}
			if s[i-1] == '\\' {
				str = str[:len(str)-1]
				str += "."
			} else {
				ret = append(ret, str)
				str = ""
			}
		default:
			str += string(ch)
		}
	}
	return append(ret, str), nil
}
