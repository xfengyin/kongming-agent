// Package plugin loader.go 提供 .so 动态插件加载占位实现。
//
// 当前实现为最小可用骨架：
//   - 仅在 Linux 平台（runtime.GOOS == "linux"）尝试调用 stdlib plugin.Open。
//   - 其它平台（darwin/freebsd 的 plugin 包本身仅"部分实现"；windows 不支持）直接返回 error。
//   - 真正加载成功后会查找约定的 "NewHandler" 符号（func() Handler）。
//
// 业务侧可在本文件之后注入：符号名校验、版本检查、白名单、签名验证等。
package plugin

import (
	"errors"
	"fmt"
	"plugin"
	"runtime"
)

// loadSoSymbolName 是 .so 中约定的导出符号名；Handler 构造函数必须使用此名导出。
const loadSoSymbolName = "NewHandler"

// LoadSoPlugin 从 .so 路径加载一个 Handler 插件。
//
// 返回：
//   - 成功：plugin.Handler 实例与 nil。
//   - 失败：nil 与具体 error；常见原因：跨平台、文件不存在、符号缺失、类型不匹配。
//
// 注意：Go 的 plugin 包要求 host 与 .so 必须用**完全相同**的 Go toolchain、依赖版本与 -trimpath 设置编译，
// 否则加载会因类型 / 字符串不匹配失败。这部分由运维侧负责；本函数只负责转发错误。
func LoadSoPlugin(path string) (Handler, error) {
	if path == "" {
		return nil, errors.New("plugin: empty path")
	}
	// Go stdlib plugin 仅在 Linux / FreeBSD / macOS 实现；其他平台直接拒。
	// 在已实现平台上若 cgo 关闭也会回退到 stub；这里统一在运行时做兜底校验。
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("plugin: .so loading is not supported on %s", runtime.GOOS)
	}

	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("plugin: open %s: %w", path, err)
	}
	sym, err := p.Lookup(loadSoSymbolName)
	if err != nil {
		return nil, fmt.Errorf("plugin: lookup %s in %s: %w", loadSoSymbolName, path, err)
	}
	// 约定符号类型为 func() Handler；若 .so 作者导出其它签名会在这里失败。
	ctor, ok := sym.(func() Handler)
	if !ok {
		return nil, fmt.Errorf("plugin: symbol %s in %s has unexpected type %T", loadSoSymbolName, path, sym)
	}
	return ctor(), nil
}
