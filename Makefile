# LanPanel 构建脚本
# 本机 Go 不在 PATH 时：make GO=~/sdk/go/bin/go
GO      ?= go
VERSION ?= $(shell cat VERSION 2>/dev/null || echo 0.0.0)
LDFLAGS := -s -w -X main.Version=$(VERSION)
OUT     := dist

.PHONY: all web build run dev-web test cross clean docker docker-save docker-push ipk fpk bins FORCE

all: web build

web: ## 构建前端（输出到 web/dist，被 go:embed 嵌入）
	cd web && npm ci && npm run build

build: ## 构建本机二进制
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel ./cmd/lanpanel

run: ## 运行后端（前端需先 make web，或另开终端 make dev-web）
	$(GO) run ./cmd/lanpanel -listen :3080 -data ./data

dev-web: ## 前端热更新开发服务器，接口代理到 :3080
	cd web && npm run dev

test:
	$(GO) vet ./... && $(GO) test ./...

# 交叉编译：Docker / 飞牛 / OpenWrt 常见架构
cross: web
	@mkdir -p $(OUT)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel-linux-amd64 ./cmd/lanpanel
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel-linux-arm64 ./cmd/lanpanel
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel-linux-armv7 ./cmd/lanpanel
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel-linux-mipsle ./cmd/lanpanel
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/lanpanel-linux-mips ./cmd/lanpanel
	@ls -lh $(OUT)

# ---- OpenWrt ipk / 飞牛 fpk ----
# 各架构的 Go 编译参数
GOENV_amd64  := GOARCH=amd64
GOENV_arm64  := GOARCH=arm64
GOENV_armv7  := GOARCH=arm GOARM=7
GOENV_mipsle := GOARCH=mipsle GOMIPS=softfloat
GOENV_mips   := GOARCH=mips GOMIPS=softfloat

$(OUT)/bin/lanpanel-linux-%: FORCE
	@test -f web/dist/index.html || { echo "请先执行 make web 构建前端"; exit 1; }
	@mkdir -p $(OUT)/bin
	CGO_ENABLED=0 GOOS=linux $(GOENV_$*) $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/lanpanel

# 例：make ipk IPK_ARCHS="arm64 mipsle" UPX=1
IPK_ARCHS ?= amd64 arm64 armv7 mipsle mips
ipk: $(foreach a,$(IPK_ARCHS),$(OUT)/bin/lanpanel-linux-$(a))
	@for a in $(IPK_ARCHS); do \
		$(GO) run ./deploy/pack ipk -arch $$a -bin $(OUT)/bin/lanpanel-linux-$$a -version $(VERSION) -out $(OUT) $(if $(UPX),-upx) || exit 1; \
	done

# 需要官方 fnpack（在 PATH 中，或 make fpk FNPACK=/path/to/fnpack）
FPK_ARCHS ?= amd64 arm64
fpk: $(foreach a,$(FPK_ARCHS),$(OUT)/bin/lanpanel-linux-$(a))
	@for a in $(FPK_ARCHS); do \
		$(GO) run ./deploy/pack fpk -arch $$a -bin $(OUT)/bin/lanpanel-linux-$$a -version $(VERSION) -out $(OUT) $(if $(FNPACK),-fnpack $(FNPACK)) || exit 1; \
	done

FORCE:

# ---- Docker ----
IMAGE     ?= lanpanel
PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
# 基础镜像仓库，国内网络：make docker BASE_REGISTRY=docker.m.daocloud.io
BASE_REGISTRY ?= docker.io
BUILD_ARGS := --build-arg VERSION=$(VERSION) --build-arg BASE_REGISTRY=$(BASE_REGISTRY)

docker: ## 构建本机架构镜像
	docker build $(BUILD_ARGS) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

# 构建指定架构并导出为文件，拷到 NAS 上 docker load（适合没有镜像仓库的情况）
# 例：make docker-save ARCH=arm64
ARCH ?= amd64
docker-save:
	@mkdir -p $(OUT)
	docker buildx build --platform linux/$(ARCH) $(BUILD_ARGS) -t $(IMAGE):latest --load .
	docker save $(IMAGE):latest | gzip > $(OUT)/lanpanel-docker-$(ARCH).tar.gz
	@ls -lh $(OUT)/lanpanel-docker-$(ARCH).tar.gz

# 多架构构建并推送到镜像仓库，例：make docker-push IMAGE=registry.example.com/me/lanpanel
docker-push:
	docker buildx build --platform $(PLATFORMS) $(BUILD_ARGS) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --push .

clean:
	rm -rf $(OUT)
