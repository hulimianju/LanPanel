# LanPanel

局域网导航面板 + 设备发现。类似 Sun-Panel 的书签面板，额外提供局域网设备嗅探：用 ARP、mDNS、SSDP、NetBIOS 和端口指纹找出网内设备与服务，书签可以绑定到设备的 MAC 地址，设备 IP 变化后自动更新。**完全离线运行，不依赖外网。**

> 当前进度：**阶段 1（导航面板）、阶段 2（设备发现）已完成**，Docker 镜像、飞牛 fpk、OpenWrt ipk 均可构建。IP 自动跟随（阶段 3）开发中。
> 设计稿：`design/` 目录。

## 特点

- **单文件部署**：Go 编写，前端嵌入二进制，无运行时依赖；可运行在 x86、ARM、MIPS 路由器上
- **占用低**：二进制约 8MB，常驻内存约 10–20MB；数据存为单个 JSON 文件，方便备份
- **专业的界面**：深浅色主题，桌面与手机自适应
- **壁纸深浅自适应**：上传壁纸后自动分析亮度，决定面板用深色调还是浅色调；遮罩、模糊可调，并实时显示文字对比度，低于 4.5:1 时给出一键修正
- **内外网地址**：每个应用可同时填写内网与外网地址，按访问者 IP 自动选择，也可手动切换
- **自动抓取**：填入地址后自动获取网站标题与图标（兼容局域网自签证书）

## 快速开始

```bash
./lanpanel -listen :3080 -data ./data
```

浏览器打开 `http://<设备IP>:3080`，首次访问会引导创建管理员账号。

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `-listen` | `LANPANEL_LISTEN` | `:3080` | 监听地址 |
| `-data` | `LANPANEL_DATA` | `./data` | 数据目录（`data.json` 与上传的图片） |
| `-version` | | | 显示版本 |

## 设备发现

「设备发现」页会列出局域网里的所有设备，包括 IP、MAC、厂商、主机名、类型、开放的服务，以及 IP 变化历史。发现到的网页服务可以一键加入面板。

**发现流程**（完整扫描一个 /24 网段通常需要 10–40 秒）：

| 阶段 | 方式 | 得到什么 |
|---|---|---|
| 主机发现 | ARP 扫描；ICMP Ping；非直连网段用 TCP 探测 | 在线设备、MAC 地址 |
| 协议广播 | mDNS / Bonjour、SSDP / UPnP、WS-Discovery、NetBIOS、SNMP、DHCP 租约、反向 DNS | 主机名、型号、厂商、设备类型、设备自己声明的管理页 |
| 端口扫描 | 对在线主机扫描规则库里的端口（默认约 80 个） | 开放端口 |
| 指纹识别 | 抓取页面标题、响应头、网站图标哈希，以及 TCP 服务的欢迎信息，再与规则匹配 | 服务名称与访问地址 |

- **设备身份**：以 MAC 地址标识设备，IP 变化会记入历史。跨网段设备拿不到 MAC，按 IP 记录。
- **路由器与交换机**：
  - 默认网关、UPnP 网关、路由系统的管理页都能说明它是路由器。
  - 可网管的交换机和 AP 依靠 SNMP：设备描述和「服务层级」字段可以区分二层交换机和三层路由器。
  - 非网管的傻瓜交换机没有 IP，无法被发现。
- **定时扫描**：
  - 默认每 30 分钟完整扫描一次。
  - 每 5 分钟快速扫描一次，只做主机发现和协议广播，用来及时发现 IP 变化。
  - 可以在「扫描任务」页调整间隔、并发数、网段和协议。
- **权限**：
  - 有原始套接字权限时（root，或 Docker 中授予了 `NET_RAW`），通过主动发送 ARP 请求判断设备在线，结果最准确。
  - 没有权限时，退回读取系统邻居表。这种方式下，设备离线后要过一段时间才会显示为离线。
- **小内存设备**：总内存小于 768MB（如路由器）时，自动把并发数降到 64。

### 端口规则

内置 110 多条规则，覆盖以下类别（见 `internal/rules/builtin.yaml`）：
- NAS：群晖、飞牛、威联通、绿联、TrueNAS、Unraid
- 虚拟化：PVE、ESXi、Portainer、1Panel、宝塔
- 路由与网络：OpenWrt、爱快、各品牌路由器与网管交换机
- 影音与下载
- 智能家居、运维工具

可以在「端口规则」页启用或停用规则、修改内置规则（随时可以恢复默认）、新建自定义规则，也能导入导出 YAML。编辑规则时可以输入一个 IP 实时测试，界面会显示该端口的页面标题、Server 响应头、欢迎信息和网站图标哈希，方便编写匹配条件。

带匹配条件的网页规则**不局限于声明的端口**。比如 Jellyfin 被映射到 18096 端口，依然能靠页面标题识别出来。

## 安装包

| 形式 | 构建 | 安装 |
|---|---|---|
| 飞牛 fnOS | `make fpk`（需官方 fnpack） | 应用中心 → 手动安装 `lanpanel_<版本>_<x86\|arm>.fpk` |
| OpenWrt | `make ipk`（无需 SDK） | `opkg install lanpanel_<版本>-1_<架构>.ipk`，LuCI「服务」菜单出现入口 |
| Docker | `make docker` | 见下方「Docker 部署」 |

打包步骤、架构对照表、权限设计与排查方法见 **[docs/packaging.md](docs/packaging.md)**。

## Docker 部署

```bash
# 构建镜像（国内网络加上 BASE_REGISTRY 指定可用的镜像加速站）
make docker BASE_REGISTRY=docker.m.daocloud.io

# 在 Linux 主机上运行
cd deploy/docker && docker compose up -d
```

镜像约 18MB，运行时内存约 10MB。**必须使用 host 网络**（compose 文件已配置）：在 bridge 网络下，所有访问者都显示为 Docker 网关 IP，内外网自动判断会失效；设备发现也收不到局域网广播。

没有镜像仓库时，可以导出为文件，拷到 NAS 上导入：

```bash
make docker-save ARCH=amd64 BASE_REGISTRY=docker.m.daocloud.io   # 或 ARCH=arm64，生成 dist/lanpanel-docker-<架构>.tar.gz
# 在 NAS 上：
docker load < lanpanel-docker-amd64.tar.gz
```

推送多架构镜像（amd64 / arm64 / armv7）到仓库：`make docker-push IMAGE=<仓库地址>/lanpanel`

## 使用说明

- **编辑面板**：登录后点击右上角「编辑」进入编辑模式。卡片可以拖动排序，也能跨分组拖动；拖动分组左侧的手柄可以调整分组顺序；双击分组名可以重命名
- **添加应用**：填写内网地址后点击「自动获取」，会自动抓取网站的标题和图标。只填 `192.168.1.23:5666` 也可以，会自动补全 `http://`
- **搜索**：按 `/` 聚焦搜索框。只匹配到一个应用时，回车直接打开；否则回车用搜索引擎搜索
- **内网 / 外网**：「自动」模式按访问者 IP 判断是否在局域网内，也支持 CGNAT 网段（Tailscale 等）。访客可以在右上角手动切换，选择只保存在自己的浏览器里
- **壁纸**：在「设置 → 外观」中上传。系统会检测壁纸亮度并自动选择面板色调；遮罩的颜色与色调一致（深色调叠黑色，浅色调叠白色）。预览区与首页使用同一套样式，看到的效果就是首页的实际效果
- **访客模式**：开启后，未登录的访客可以只读浏览面板；本机状态卡片只有登录后才显示
- **备份**：「设置 → 备份与迁移」可以导出和导入 JSON，导入支持追加和覆盖两种方式

## 开发

依赖 Go 1.22+ 与 Node.js 20+。

```bash
make web          # 构建前端
make run          # 启动后端 :3080
make dev-web      # 另开终端：前端热更新 http://localhost:5173（接口代理到 :3080）
make test         # 单元测试
make cross        # 交叉编译 amd64 / arm64 / armv7 / mipsle / mips
make ipk          # OpenWrt 安装包（5 个架构）
make fpk          # 飞牛安装包（x86 / arm，需要 fnpack）
```

### 目录结构

```
cmd/lanpanel/        程序入口
internal/api/        HTTP 接口、认证中间件、静态资源
internal/store/      JSON 文件存储（原子写入 + .bak 备份）
internal/auth/       密码哈希、签名会话、登录限流
internal/icon/       网站标题与图标抓取
internal/sysinfo/    本机 CPU / 内存 / 网速采集
internal/discovery/  设备发现：ARP / mDNS / SSDP / NetBIOS / WSD / SNMP、端口扫描、指纹识别、设备库
internal/rules/      端口与指纹规则（builtin.yaml）及匹配引擎
internal/oui/        MAC 厂商库（由 Wireshark manuf 生成，见 gen/）
web/                 Vue 3 + TypeScript + Tailwind 前端
  src/styles/main.css    设计变量：界面主题 + 面板表面（壁纸深浅适配）
  src/lib/luminance.ts   壁纸亮度采样与对比度计算
design/              UI 设计稿（画布源文件）
assets/icon/         应用图标（SVG 源文件与各尺寸 PNG / ICO）
deploy/docker/       docker-compose
deploy/fnos/         飞牛 fpk 模板（manifest、生命周期脚本、桌面入口）
deploy/openwrt/      OpenWrt ipk 文件（procd 服务、UCI 配置、LuCI 入口）
deploy/pack/         ipk / fpk 打包工具
docs/                打包与安装文档
```

### 主题与壁纸的样式体系

样式分为两套相互独立的变量：

1. **界面主题** `html[data-theme=light|dark]`：用于管理页面、弹窗和表单，由「设置 → 界面主题」控制，可以跟随系统
2. **面板表面** `.lp-surface[data-tone][data-wp]`：用于首页叠在壁纸之上的文字和毛玻璃卡片
   - `data-tone`：由壁纸亮度决定，与界面主题无关
   - `data-wp=image`：图片壁纸时启用毛玻璃，卡片自带半透明底色，保证在任意图片上都有足够的对比度
   - 浏览器不支持 `backdrop-filter` 时，自动提高卡片的不透明度

组件只使用语义化的颜色类：管理页用 `bg-surface`、`text-fg-2` 这类，面板用 `lp-glass`、`text-s-fg` 这类。新增主题或色调时，只需要改 `main.css`。

## 安全

- 密码使用 bcrypt 存储；会话采用 HMAC 签名，修改密码后旧会话全部失效
- 同一 IP 10 分钟内登录失败 8 次，会被锁定
- 修改类接口只接受 JSON 或上传表单，配合 SameSite Cookie 防御 CSRF
- 链接禁止 `javascript:`、`data:` 等可执行脚本的协议；上传的 SVG 以沙箱方式返回，其中的脚本不会执行
