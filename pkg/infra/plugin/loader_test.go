// Package plugin loader_test.go 覆盖 .so 加载占位函数的错误路径。
// 由于真实 .so 需要 -buildmode=plugin 编译产物，本文件不覆盖成功分支（参见 LoadSoPlugin 注释）。
package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSoPlugin_EmptyPath_ReturnsError(t *testing.T) {
	h, err := LoadSoPlugin("")
	assert.Nil(t, h)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestLoadSoPlugin_NonexistentFile_ReturnsError(t *testing.T) {
	h, err := LoadSoPlugin("/this/path/does/not/exist/plugin.so")
	assert.Nil(t, h)
	require.Error(t, err)
	// 在非支持平台上，错误文案以 "not supported" 开头；
	// 在支持平台上以 "open" / "lookup" 开头。两者都说明已尝试加载。
	msg := err.Error()
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		assert.Contains(t, msg, "plugin:")
	} else {
		assert.Contains(t, msg, "not supported")
	}
}

func TestLoadSoPlugin_NonSOFile_OpenSucceedsButLookupFails(t *testing.T) {
	// 临时写一个非 ELF 的 .so 文件，触发 plugin.Open 后 Lookup 失败路径。
	// 仅在 plugin 实现可用的平台测试。
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "darwin" {
		t.Skipf(".so loading not supported on %s", runtime.GOOS)
	}
	tmp := t.TempDir()
	fakeSO := filepath.Join(tmp, "fake.so")
	require.NoError(t, writeFile(fakeSO, []byte("not a real ELF")))

	h, err := LoadSoPlugin(fakeSO)
	assert.Nil(t, h)
	require.Error(t, err)
	// "fake" 字节流可能让 plugin.Open 失败（格式不识别），也可能在 Open 之后 Lookup "NewHandler" 失败。
	// 两种都被允许：关键是返回非 nil error 且 h == nil。
	assert.Nil(t, h)
}

func TestLoadSoPlugin_UnsupportedPlatform_ReturnsError(t *testing.T) {
	// windows / 其他无 plugin 实现的 OS：直接返回 "not supported"。
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		t.Skipf("current platform %s supports plugin", runtime.GOOS)
	}
	h, err := LoadSoPlugin(filepath.Join(t.TempDir(), "x.so"))
	assert.Nil(t, h)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// writeFile 是 os.WriteFile 的薄包装，便于上溯排查。
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
