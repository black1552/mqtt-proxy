//go:build ignore

// gen.go 是 main.go 中 //go:generate 指令的辅助脚本，把发布构建拆成两步：
// 第一步编译内嵌的管理后台前端（admin/ → admin/dist），
// 第二步用 go build 交叉编译 Linux 发布二进制；Windows 为可选步骤。
// 也可脱离 go generate 直接运行：go run gen.go [frontend|linux|windows]...
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// releaseDir 是发布二进制的输出目录（已在 .gitignore 中忽略）。
const releaseDir = "release"

func main() {
	steps := os.Args[1:]
	if len(steps) == 0 {
		// 默认执行文档化的两步：前端 + Linux。
		steps = []string{"frontend", "linux"}
	}
	for _, s := range steps {
		switch s {
		case "frontend":
			buildFrontend()
		case "linux":
			buildBinary("linux", "amd64", "mqtt-proxy-linux-amd64")
		case "windows":
			buildBinary("windows", "amd64", "mqtt-proxy-windows-amd64.exe")
		default:
			fmt.Fprintf(os.Stderr, "未知步骤 %q，支持 frontend / linux / windows\n", s)
			os.Exit(2)
		}
	}
}

// buildFrontend 编译管理后台前端；首次构建时先安装依赖。
func buildFrontend() {
	fmt.Println("==> [1/2] building admin frontend")
	if err := os.Chdir("admin"); err != nil {
		fail("进入 admin 目录失败", err)
	}
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		run(npmCmd(), "install")
	}
	run(npmCmd(), "run", "build")
	if err := os.Chdir(".."); err != nil {
		fail("返回项目根目录失败", err)
	}
}

// npmCmd 返回 npm 可执行文件名。Windows 上 npm 是 npm.cmd 批处理包装，
// CreateProcess 无法直接执行，需显式带扩展名交给 cmd 解析。
func npmCmd() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

// buildBinary 用 go build 交叉编译单个平台的发布二进制。
// SQLite 驱动为纯 Go 实现，统一关闭 CGO 以获得无依赖的静态二进制。
func buildBinary(goos, goarch, name string) {
	fmt.Printf("==> building %s/%s -> %s/%s\n", goos, goarch, releaseDir, name)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		fail("创建输出目录失败", err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w",
		"-o", filepath.Join(releaseDir, name), ".")
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	if err != nil {
		fail("go build 失败", err)
	}
}

// run 执行外部命令并把输出透传到当前终端，失败即退出。
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail(name+" "+fmt.Sprint(args), err)
	}
}

func fail(msg string, err error) {
	fmt.Fprintln(os.Stderr, "error:", msg, ":", err)
	os.Exit(1)
}
