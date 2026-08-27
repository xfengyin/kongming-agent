// Kongming 孔明军师系统 - 命令行入口
// 运筹帷幄之中，决胜千里之外

package main

import (
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
