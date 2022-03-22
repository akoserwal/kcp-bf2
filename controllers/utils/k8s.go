package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func EncodeKubernetesName(name string, length int) string {
	encodedName := strings.Builder{}

	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '.' {
			//allowed in name.
			encodedName.WriteRune(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			//Apply toLower encoding.
			encodedName.WriteString(fmt.Sprintf(".%c", unicode.ToLower(ch)))
		} else {
			//Apply character-code encoding.
			encodedName.WriteString(fmt.Sprintf(".%X", int(ch)))
		}
	}
	return fmt.Sprintf("%."+strconv.Itoa(length)+"s", encodedName.String())
}
