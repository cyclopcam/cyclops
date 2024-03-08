package kibi

import "fmt"

func Bytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%v bytes", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%v KB", b/1024)
	} else if b < 1024*1024*1024 {
		return fmt.Sprintf("%v MB", b/1024/1024)
	} else {
		return fmt.Sprintf("%v GB", b/1024/1024/1024)
	}
}
