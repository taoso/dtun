// +build linux
package dtun

import "fmt"

const ipcmd = "/usr/sbin/ip"

func init() {
	fmt.Println("linux", ipcmd)
}
