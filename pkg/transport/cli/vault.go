// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming vault` 子命令：锦囊库管理。
//
// 子命令：
//
//	list           列出全部锦囊
//	exec <id>      执行指定锦囊（需 --data <json>）
//
// Stage 4 阶段无真实 Vault 注入，--dry-run 验证参数解析。
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// ErrVaultMissingID 当 exec 子命令未传 id 时返回。
var ErrVaultMissingID = errors.New("vault exec: id is required")

// ErrVaultMissingData 当 exec 子命令未传 --data 时返回。
var ErrVaultMissingData = errors.New("vault exec: --data <json> is required")

// newVaultCmd 构造 `kongming vault` 子命令。
func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Vault (锦囊) operations",
		Long: `锦囊库管理：
  list             列出全部锦囊
  exec <id>        执行指定锦囊（--data <json> 传入 payload）`,
	}

	cmd.AddCommand(newVaultListCmd())
	cmd.AddCommand(newVaultExecCmd())
	return cmd
}

// newVaultListCmd 实现 `kongming vault list`。
func newVaultListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all jinnang in the vault",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "vault.list",
					"dry_run": true,
					"jinnang": []model.Jinnang{},
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Vault == nil {
				return ErrServiceNotWired
			}
			jinnangs, err := svc.Vault.ListJinnang()
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action":  "vault.list",
				"jinnang": jinnangs,
			})
		},
	}
}

// newVaultExecCmd 实现 `kongming vault exec <id> --data <json>`。
func newVaultExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <id>",
		Short: "Execute a jinnang with the given data payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" {
				return ErrVaultMissingID
			}
			data, _ := cmd.Flags().GetString("data")
			if data == "" {
				return ErrVaultMissingData
			}
			// 校验 JSON 合法性：失败时提前报错，避免把非法 payload 送到 service。
			if err := validateJSON(data); err != nil {
				return fmt.Errorf("vault exec: invalid --data JSON: %w", err)
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			input := model.JinnangInput{Data: json.RawMessage(data)}

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "vault.exec",
					"dry_run": true,
					"id":      id,
					"input":   input,
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Vault == nil {
				return ErrServiceNotWired
			}
			out, err := svc.Vault.Execute(cmd.Context(), id, input)
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action": "vault.exec",
				"id":     id,
				"output": out,
			})
		},
	}
	// --data 接收原始 JSON 字符串。
	cmd.Flags().String("data", "", "JSON payload (required)")
	return cmd
}

// validateJSON 校验字符串是否为合法 JSON（仅做格式校验，不解析具体结构）。
func validateJSON(s string) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return nil
}
