// +build darwin
package dtun

import "fmt"

const ipcmd = "/usr/local/bin/ip"

func init() {
	fmt.Println("darwin", ipcmd)
}
