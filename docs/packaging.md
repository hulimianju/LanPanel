# 打包与安装

LanPanel 提供四种分发形式：

| 形式 | 适用 | 构建命令 | 产物 |
|---|---|---|---|
| 单文件程序 | 任意 Linux / macOS / Windows | `make build` 或 `make cross` | `dist/lanpanel*` |
| Docker 镜像 | 装了 Docker 的 Linux 主机 | `make docker` | `lanpanel:latest` |
| 飞牛 fpk | 飞牛 fnOS（x86 / ARM） | `make fpk` | `dist/lanpanel_<版本>_<x86\|arm>.fpk` |
| OpenWrt ipk | OpenWrt / iStoreOS / ImmortalWrt 等 | `make ipk` | `dist/lanpanel_<版本>-1_<架构>.ipk` |

版本号统一写在项目根目录的 `VERSION` 文件里。所有安装包都内置完整的前端页面，不依赖外网。

## 准备工作

所有打包方式都需要先构建前端，前端产物会被嵌入到程序里：

```bash
make web            # 需要 Node.js 20+，只在前端代码变化后需要重新执行
```

编译需要 Go 1.22 以上。本机 Go 不在 PATH 中时，可以加上 `GO=~/sdk/go/bin/go` 参数。

---

## 飞牛 fnOS 应用包（fpk）

### 1. 安装官方打包工具 fnpack

从[飞牛开发者文档](https://developer.fnnas.com/docs/cli/fnpack/)下载对应系统的 fnpack（1.2.3）：

```bash
# macOS Intel 示例；Apple 芯片把文件名换成 darwin-arm64，Linux 换成 linux-amd64
curl -L -o fnpack https://static2.fnnas.com/fnpack/fnpack-1.2.3-darwin-amd64
shasum -a 256 fnpack   # 应为 30a9f50a35e8d8d425b687881761478c3c778e9c0da3a1b59f298b666dd7a268
chmod +x fnpack && sudo mv fnpack /usr/local/bin/
```

各版本的 SHA-256 校验值：

| 文件 | SHA-256 |
|---|---|
| fnpack-1.2.3-linux-amd64 | `54b97fa7b70968c4d05c79840f5daeff508957d0bb2062fdb0376d00d9615c93` |
| fnpack-1.2.3-darwin-amd64 | `30a9f50a35e8d8d425b687881761478c3c778e9c0da3a1b59f298b666dd7a268` |
| fnpack-1.2.3-darwin-arm64 | `d40cb00896cb2a5d211357d255750ed0cbe7f2d141df671c2b717afb4e74bf77` |

### 2. 打包

```bash
make fpk                        # 同时打 x86 与 arm 两个包
make fpk FPK_ARCHS=amd64        # 只打 x86
make fpk FNPACK=/path/to/fnpack # fnpack 不在 PATH 中时指定路径
```

`make fpk` 依次做三件事：
1. 交叉编译 linux/amd64、linux/arm64 程序。
2. 由 `deploy/pack` 组装出标准的飞牛应用目录，位于 `dist/fnos/lanpanel-<平台>/`。
3. 调用 `fnpack build` 生成 `.fpk`。

fnpack 会校验必要的文件和格式，还会把 `app.tgz` 的 MD5 写入 manifest 的 `checksum` 字段。

没有 fnpack 时，第 2 步照常完成，你可以把组装好的目录拷到飞牛上，再执行 `fnpack build --directory <目录>`。

### 3. 安装

在飞牛的「应用中心 → 手动安装」里选择 `.fpk` 文件。x86 设备选 `_x86.fpk`，ARM 设备选 `_arm.fpk`。安装完成后，桌面上会出现 LanPanel 图标，点击即可打开 `http://NAS的IP:3080`。

### 应用包结构

模板位于 `deploy/fnos/lanpanel/`：

```
manifest.in          # 清单模板：{{VERSION}} {{PLATFORM}} {{CHANGELOG}} 在打包时替换
config/privilege     # 运行身份
config/resource      # 资源声明（LanPanel 不需要共享目录）
cmd/main             # start / stop / status，含守护循环
cmd/*_init|callback  # 其余生命周期脚本（无需操作）
wizard/              # 安装向导（未使用，但 fnpack 要求目录存在）
app/ui/config        # 桌面图标入口：type=url，端口 3080
```

打包时还会自动加入以下文件：
- `app/bin/lanpanel`：程序本体
- `ICON.PNG`、`ICON_256.PNG`：应用中心使用的图标
- `app/ui/images/icon_{64,256}.png`：桌面图标

关键设计：

- **权限**：遵循飞牛「长期运行的服务应以非 root 身份运行」的要求。
  - 生命周期脚本以 root 运行（`run-as=root`），只做一件特权准备：用 `setcap cap_net_raw+ep` 给程序授予原始套接字能力。
  - 之后用 `setpriv` 以专用用户 `lanpanel` 启动服务。
  - 实测服务进程的有效权限只有 `CAP_NET_RAW` 这一项。
  - 系统里没有 `setcap` 时，设备发现退回读取邻居表的方式，功能不受影响。
- **守护**：飞牛不会托管应用进程，崩溃后不会自动拉起。所以 `cmd/main` 自带守护循环：程序异常退出后按 2 秒、4 秒……最长 60 秒的间隔重启。
- **数据**：面板配置和设备库保存在 `$TRIM_PKGVAR/data`，日志在 `$TRIM_PKGVAR/lanpanel.log`，超过 5MB 自动轮转。
- **端口**：`service_port=3080`，`checkport=true`。端口被占用时，飞牛会拒绝启动。要修改端口，需要同时修改 `manifest.in` 的 `service_port` 和 `app/ui/config` 的 `port`，然后重新打包。

### 排查

```bash
# 在飞牛上通过 SSH 查看
tail -f /var/apps/lanpanel/var/lanpanel.log
getcap /var/apps/lanpanel/target/bin/lanpanel   # 应输出 cap_net_raw=ep
```

---

## OpenWrt 安装包（ipk）

### 1. 打包

```bash
make ipk                                   # 打全部 5 个架构
make ipk IPK_ARCHS="arm64 mipsle"          # 只打指定架构
make ipk UPX=1                             # 用 upx 压缩程序（需先安装 upx），体积约减少 60%，适合小闪存路由器
```

产物位于 `dist/`，文件名里的架构对应关系如下：

| `IPK_ARCHS` 取值 | 安装包文件名 | 适用设备（`DISTRIB_ARCH`） | 常见机型 |
|---|---|---|---|
| `amd64` | `_x86_64.ipk` | `x86_64` | 软路由、虚拟机 |
| `arm64` | `_aarch64.ipk` | `aarch64_*` | MT7981/7986、RK3568/3588、树莓派 4/5 |
| `armv7` | `_arm_cortex-a7.ipk` | `arm_cortex-a*` | IPQ40xx、MT7629、树莓派 2/3（32 位系统） |
| `mipsle` | `_mipsel_24kc.ipk` | `mipsel_*` | MT7621、MT7628 |
| `mips` | `_mips_24kc.ipk` | `mips_*` | AR/QCA 系列（如 QCA9563） |

不确定路由器是哪种架构时，在路由器上执行下面的命令查看：

```bash
. /etc/openwrt_release && echo $DISTRIB_ARCH
```

`ipk` 由 `deploy/pack` 直接生成，格式与 OpenWrt 官方的 `ipkg-build` 一致：外层是一个 tar.gz，里面包含 `debian-binary`、`control.tar.gz`、`data.tar.gz`，所有文件属主都是 root。生成过程不需要 OpenWrt SDK，在 macOS、Linux、Windows 上都能打包。

### 2. 安装

```bash
scp dist/lanpanel_0.2.0-1_aarch64.ipk root@192.168.1.1:/tmp/
ssh root@192.168.1.1 opkg install /tmp/lanpanel_0.2.0-1_aarch64.ipk
```

也可以在 LuCI 的「系统 → 软件包 → 上传软件包」里安装。

安装完成后：
- 服务会自动启动，并设置为开机自启。它由 procd 托管，崩溃后自动重启。
- LuCI 的「服务」菜单里会多出 **LanPanel** 入口，点击可以跳转到 `http://路由器IP:3080`。
- 安装前会检查架构：装错架构的包会被拒绝，并提示正确的架构。

### 配置

配置文件是 `/etc/config/lanpanel`：

```
config lanpanel 'main'
	option enabled '1'
	option listen ':3080'
	option data '/etc/lanpanel'
```

修改后执行 `/etc/init.d/lanpanel restart` 生效。

设备库在每次扫描后都会写入磁盘。如果闪存较小，或者担心写入寿命，建议把 `data` 改到 U 盘，例如 `/mnt/sda1/lanpanel`；同时可以在「扫描任务」页把快速扫描的间隔调大。

### 包含的文件

```
/usr/bin/lanpanel                                   程序
/etc/init.d/lanpanel                                procd 服务脚本
/etc/config/lanpanel                                UCI 配置（升级时保留）
/www/luci-static/resources/view/lanpanel.js         LuCI 页面
/usr/share/luci/menu.d/luci-app-lanpanel.json       LuCI 菜单
/usr/share/rpcd/acl.d/luci-app-lanpanel.json        LuCI 权限
```

### 兼容性

- 适用于 OpenWrt 21.02–24.10，以及基于它们的 iStoreOS、ImmortalWrt 等使用 opkg 的系统。
- OpenWrt 25.x 改用了 apk 包管理器，这里的 ipk 暂不适用。可以直接把单文件程序（`make cross` 的产物）复制到 `/usr/bin/`，再手动放置 `deploy/openwrt/files/` 下的文件。
- 在路由器上以 root 运行，设备发现直接使用原始 ARP，结果最准确。同时会读取 dnsmasq 的 DHCP 租约，获取最准确的主机名。

---

## 应用图标

图标源文件是 `assets/icon/lanpanel.svg`：靛蓝色圆角底板上，三张应用卡片加一组雷达波纹，分别代表「导航面板」和「设备发现」。已导出的文件如下：

| 文件 | 用途 |
|---|---|
| `lanpanel-{16,32,48,64,128,256,512,1024}.png` | 通用尺寸 |
| `lanpanel-180.png` | iOS「添加到主屏幕」 |
| `lanpanel-192.png` / `lanpanel-512.png` | 安卓与 PWA |
| `lanpanel.ico` | Windows / 旧版浏览器 favicon（内含 16–256） |

网页端的图标文件位于 `web/public/`：`favicon.svg`、`favicon.ico`、`apple-touch-icon.png`、`icon-192.png`、`icon-512.png`、`site.webmanifest`。飞牛包的 64 和 256 尺寸图标由 `make fpk` 自动放入对应位置。

修改 SVG 后需要重新导出 PNG。任选一种工具即可，例如：

```bash
for n in 16 32 48 64 128 180 192 256 512 1024; do
  rsvg-convert -w $n -h $n assets/icon/lanpanel.svg -o assets/icon/lanpanel-$n.png
done
```
