package kibi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var DigitRegex = regexp.MustCompile(`\d+`)
var ErrInvalidByteSizeString = fmt.Errorf("Invalid byte size string")

func Bytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%v bytes", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%v KB", b/1024)
	} else if b < 1024*1024*1024 {
		return fmt.Sprintf("%v MB", b/1024/1024)
	} else if b < 1024*1024*1024*1024 {
		return fmt.Sprintf("%v GB", b/1024/1024/1024)
	} else if b < 1024*1024*1024*1024*1024 {
		return fmt.Sprintf("%v TB", b/1024/1024/1024/1024)
	} else {
		return fmt.Sprintf("%v PB", b/1024/1024/1024/1024/1024)
	}
}

// We support suffixes 'mb', 'kb', 'gb', etc.
// We also support suffixes of just the letter, eg 'm', 'g', etc.
// Examples:
// 123 m -> 123*1024*1024
// 123 mb -> 123*1024*1024
// 123 GB -> 123*1024*1024*1024
// 123 T -> 123*1024*1024*1024*1024
// 123 P -> 123*1024*1024*1024*1024*1024
func Parse(v string) (int64, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	digits := DigitRegex.FindString(v)
	if digits == "" {
		return 0, ErrInvalidByteSizeString
	}
	suffix := strings.TrimSpace(v[len(digits):])
	multiplier := int64(1)
	if suffix == "bytes" {
	} else if suffix == "kb" || suffix == "k" {
		multiplier = 1024
	} else if suffix == "mb" || suffix == "m" {
		multiplier = 1024 * 1024
	} else if suffix == "gb" || suffix == "g" {
		multiplier = 1024 * 1024 * 1024
	} else if suffix == "tb" || suffix == "t" {
		multiplier = 1024 * 1024 * 1024 * 1024
	} else if suffix == "pb" || suffix == "p" {
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	} else if suffix != "" {
		return 0, ErrInvalidByteSizeString
	}
	if value, err := strconv.ParseInt(digits, 10, 64); err != nil {
		return 0, err
	} else {
		return value * multiplier, nil
	}
}
