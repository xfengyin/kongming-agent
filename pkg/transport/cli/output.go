// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现通用输出工具：printJSON / printYAML / printError。
// 设计原则：
//  1. 输出目标（io.Writer）由调用方注入，便于测试通过 buffer 捕获；
//  2. 序列化失败也要把错误反馈给用户（写入 stderr），不静默吞错；
//  3. JSON 用 2 空格缩进，YAML 用 4 空格缩进，便于命令行肉眼查看。
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// printJSON 将 v 序列化为带缩进的 JSON 写入 w。
//
// 出错时把错误写入同一 w 并返回 error，便于调用方在 RunE 中传播。
// 选择 w 而非 os.Stdout 是为了支持测试与日志重定向。
func printJSON(w io.Writer, v any) error {
	// 反射：marshal 任何 struct/map/slice。
	// 缩进=2：用户友好且与 kubectl/istioctl 等主流工具风格一致。
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		// 序列化失败时追加错误信息到 w，方便 CLI 用户感知。
		fmt.Fprintf(w, "{\"error\":%q}\n", err.Error())
		return fmt.Errorf("marshal json: %w", err)
	}
	return nil
}

// printYAML 将 v 序列化为 YAML 写入 w。
//
// 出错时同样写错误到 w 并返回 error。
// YAML 适合策略/工作流等可读性要求高的场景。
func printYAML(w io.Writer, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		fmt.Fprintf(w, "error: %q\n", err.Error())
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}
	return nil
}

// printError 把 err 格式化为 CLI 友好错误并写入 w。
//
// 始终附加换行符，避免和后续输出粘连。
// 若 err 为 nil 则为空操作（便于通用调用方无脑使用）。
func printError(w io.Writer, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(w, "Error: %s\n", err.Error())
}
