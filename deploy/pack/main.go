// pack 生成 OpenWrt ipk 与飞牛 fnOS fpk 安装包。
//
//	go run ./deploy/pack ipk -arch arm64 -bin dist/bin/lanpanel-linux-arm64 -version 0.2.0 -out dist
//	go run ./deploy/pack fpk -arch amd64 -bin dist/bin/lanpanel-linux-amd64 -version 0.2.0 -out dist
//
// ipk 与 OpenWrt 的 ipkg-build 格式一致：gzip 压缩的 tar，内含 debian-binary、control.tar.gz、data.tar.gz，
// 文件属主统一为 root。用 Go 实现是为了在 macOS / Linux / Windows 上生成完全一致的结果。
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type archInfo struct {
	OpenWrtGlob  string // 与 /etc/openwrt_release 的 DISTRIB_ARCH 匹配的模式
	OpenWrtLabel string // 安装包文件名中的架构
	FnOS         string // 飞牛 manifest 的 platform（空表示不支持）
}

var arches = map[string]archInfo{
	"amd64":  {"x86_64", "x86_64", "x86"},
	"arm64":  {"aarch64_*", "aarch64", "arm"},
	"armv7":  {"arm_cortex-a*", "arm_cortex-a7", ""},
	"mipsle": {"mipsel_*", "mipsel_24kc", ""},
	"mips":   {"mips_*", "mips_24kc", ""},
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fl := flag.NewFlagSet(cmd, flag.ExitOnError)
	arch := fl.String("arch", "amd64", "架构：amd64 | arm64 | armv7 | mipsle | mips")
	bin := fl.String("bin", "", "已编译的 linux 程序路径")
	version := fl.String("version", "0.0.0", "版本号")
	out := fl.String("out", "dist", "输出目录")
	root := fl.String("root", ".", "项目根目录")
	upx := fl.Bool("upx", false, "用 upx 压缩程序（需已安装 upx，体积约减少 60%）")
	fnpack := fl.String("fnpack", os.Getenv("FNPACK"), "fnpack 路径（默认从 PATH 查找）")
	_ = fl.Parse(os.Args[2:])

	info, ok := arches[*arch]
	if !ok {
		fail("未知架构：%s", *arch)
	}
	if *bin == "" {
		fail("请用 -bin 指定已编译的程序")
	}
	binData, err := os.ReadFile(*bin)
	if err != nil {
		fail("读取程序失败：%v", err)
	}
	if *upx {
		binData = compressUPX(binData)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail("%v", err)
	}

	switch cmd {
	case "ipk":
		p, err := buildIPK(*root, info, binData, *version, *out)
		if err != nil {
			fail("生成 ipk 失败：%v", err)
		}
		report(p)
	case "fpk":
		if info.FnOS == "" {
			fail("飞牛 fnOS 只支持 amd64（x86）与 arm64（arm）")
		}
		p, err := buildFPK(*root, info, binData, *version, *out, *fnpack)
		if err != nil {
			fail("生成 fpk 失败：%v", err)
		}
		report(p)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法：pack ipk|fpk -arch <架构> -bin <程序> -version <版本> [-out dist] [-upx]")
	os.Exit(2)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "错误："+format+"\n", args...)
	os.Exit(1)
}

func report(p string) {
	st, _ := os.Stat(p)
	fmt.Printf("已生成 %s（%.1f MB）\n", p, float64(st.Size())/1024/1024)
}

func compressUPX(data []byte) []byte {
	if _, err := exec.LookPath("upx"); err != nil {
		fail("未找到 upx，请先安装（macOS: brew install upx；Debian: apt install upx-ucl）或去掉 -upx")
	}
	tmp, err := os.CreateTemp("", "lanpanel-upx-*")
	if err != nil {
		fail("%v", err)
	}
	defer os.Remove(tmp.Name())
	_, _ = tmp.Write(data)
	tmp.Close()
	cmd := exec.Command("upx", "-q", "--best", "--lzma", tmp.Name())
	if outp, err := cmd.CombinedOutput(); err != nil {
		fail("upx 压缩失败：%v\n%s", err, outp)
	}
	out, _ := os.ReadFile(tmp.Name())
	return out
}

// ---- tar 工具 ----

type entry struct {
	Name string // 以 ./ 开头的归档路径；目录以 / 结尾
	Mode int64
	Data []byte
}

func mtime() time.Time {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" { // 可重复构建
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(n, 0)
		}
	}
	return time.Now().Truncate(time.Second)
}

func tarGz(entries []entry) ([]byte, error) {
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	tw := tar.NewWriter(gz)
	t := mtime()
	for _, e := range entries {
		h := &tar.Header{Name: e.Name, Mode: e.Mode, ModTime: t, Uid: 0, Gid: 0, Uname: "root", Gname: "root", Format: tar.FormatGNU}
		if strings.HasSuffix(e.Name, "/") {
			h.Typeflag = tar.TypeDir
		} else {
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.Data))
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(e.Data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// withDirs 为文件补齐所有父目录条目，并按路径排序。
func withDirs(files []entry) []entry {
	dirs := map[string]bool{"./": true}
	for _, f := range files {
		parts := strings.Split(strings.TrimPrefix(f.Name, "./"), "/")
		for i := 1; i < len(parts); i++ {
			dirs["./"+strings.Join(parts[:i], "/")+"/"] = true
		}
	}
	out := make([]entry, 0, len(files)+len(dirs))
	for d := range dirs {
		out = append(out, entry{Name: d, Mode: 0o755})
	}
	out = append(out, files...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---- ipk ----

func buildIPK(root string, info archInfo, bin []byte, version, out string) (string, error) {
	dir := filepath.Join(root, "deploy", "openwrt")
	files := []entry{{Name: "./usr/bin/lanpanel", Mode: 0o755, Data: bin}}
	size := int64(len(bin))
	err := filepath.WalkDir(filepath.Join(dir, "files"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(dir, "files"), p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		mode := int64(0o644)
		if strings.HasPrefix(filepath.ToSlash(rel), "etc/init.d/") {
			mode = 0o755
		}
		files = append(files, entry{Name: "./" + filepath.ToSlash(rel), Mode: mode, Data: data})
		size += int64(len(data))
		return nil
	})
	if err != nil {
		return "", err
	}
	data, err := tarGz(withDirs(files))
	if err != nil {
		return "", err
	}

	pkgVersion := version + "-1"
	control := fmt.Sprintf(`Package: lanpanel
Version: %s
Source: https://github.com/hulimianju/LanPanel
SourceName: lanpanel
Section: net
URL: https://github.com/hulimianju/LanPanel
Maintainer: hulimianju
Architecture: all
Installed-Size: %d
Description:  局域网导航面板与设备发现（适用于 %s）。
 通过 ARP、mDNS、SSDP、SNMP 与端口指纹发现局域网设备和服务，并提供导航面板。
`, pkgVersion, size, info.OpenWrtLabel)
	ctl := []entry{
		{Name: "./", Mode: 0o755},
		{Name: "./control", Mode: 0o644, Data: []byte(control)},
		{Name: "./conffiles", Mode: 0o644, Data: []byte("/etc/config/lanpanel\n")},
	}
	for _, s := range []string{"preinst", "postinst", "prerm", "postrm"} {
		b, err := os.ReadFile(filepath.Join(dir, "control", s))
		if err != nil {
			return "", err
		}
		b = bytes.ReplaceAll(b, []byte("{{OPENWRT_ARCH}}"), []byte(info.OpenWrtGlob))
		ctl = append(ctl, entry{Name: "./" + s, Mode: 0o755, Data: b})
	}
	control2, err := tarGz(ctl)
	if err != nil {
		return "", err
	}
	pkg, err := tarGz([]entry{
		{Name: "./debian-binary", Mode: 0o644, Data: []byte("2.0\n")},
		{Name: "./data.tar.gz", Mode: 0o644, Data: data},
		{Name: "./control.tar.gz", Mode: 0o644, Data: control2},
	})
	if err != nil {
		return "", err
	}
	p := filepath.Join(out, fmt.Sprintf("lanpanel_%s_%s.ipk", pkgVersion, info.OpenWrtLabel))
	return p, os.WriteFile(p, pkg, 0o644)
}

// ---- fpk ----

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Name() == ".gitkeep" || d.Name() == ".DS_Store" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.HasPrefix(filepath.ToSlash(rel), "cmd/") {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func buildFPK(root string, info archInfo, bin []byte, version, out, fnpack string) (string, error) {
	tpl := filepath.Join(root, "deploy", "fnos", "lanpanel")
	icons := filepath.Join(root, "assets", "icon")
	stage, _ := filepath.Abs(filepath.Join(out, "fnos", "lanpanel-"+info.FnOS))
	_ = os.RemoveAll(stage)
	if err := copyTree(tpl, stage); err != nil {
		return "", err
	}
	// manifest
	m, err := os.ReadFile(filepath.Join(stage, "manifest.in"))
	if err != nil {
		return "", err
	}
	changelog := os.Getenv("FPK_CHANGELOG")
	if changelog == "" {
		changelog = version + "：导航面板与局域网设备发现。"
	}
	m = bytes.NewBufferString(strings.NewReplacer("{{VERSION}}", version, "{{PLATFORM}}", info.FnOS, "{{CHANGELOG}}", changelog).Replace(string(m))).Bytes()
	if err := os.WriteFile(filepath.Join(stage, "manifest"), m, 0o644); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Join(stage, "manifest.in"))
	// 程序与图标
	if err := os.MkdirAll(filepath.Join(stage, "app", "bin"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, "app", "bin", "lanpanel"), bin, 0o755); err != nil {
		return "", err
	}
	for src, dst := range map[string]string{
		"lanpanel-64.png":  "ICON.PNG",
		"lanpanel-256.png": "ICON_256.PNG",
	} {
		if err := copyFile(filepath.Join(icons, src), filepath.Join(stage, dst)); err != nil {
			return "", err
		}
		ui := map[string]string{"ICON.PNG": "icon_64.png", "ICON_256.PNG": "icon_256.png"}[dst]
		if err := copyFile(filepath.Join(icons, src), filepath.Join(stage, "app", "ui", "images", ui)); err != nil {
			return "", err
		}
	}
	_ = os.MkdirAll(filepath.Join(stage, "wizard"), 0o755)

	// 调用官方 fnpack 打包
	if fnpack == "" {
		if p, err := exec.LookPath("fnpack"); err == nil {
			fnpack = p
		}
	}
	if fnpack == "" {
		return "", fmt.Errorf("未找到 fnpack。应用目录已组装在 %s，\n可下载官方 fnpack（https://developer.fnnas.com/docs/cli/fnpack/）后执行：fnpack build --directory %s", stage, stage)
	}
	cmd := exec.Command(fnpack, "build", "--directory", stage)
	cmd.Dir = filepath.Dir(stage)
	outp, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("fnpack 执行失败：%v\n%s", err, outp)
	}
	// fnpack 把 <appname>.fpk 输出到工作目录
	built := filepath.Join(cmd.Dir, "lanpanel.fpk")
	if _, err := os.Stat(built); err != nil {
		if alt := filepath.Join(stage, "lanpanel.fpk"); fileExists(alt) {
			built = alt
		} else {
			return "", errors.New("fnpack 未生成 lanpanel.fpk：\n" + string(outp))
		}
	}
	p := filepath.Join(out, fmt.Sprintf("lanpanel_%s_%s.fpk", version, info.FnOS))
	return p, os.Rename(built, p)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
