# LanPanel 多阶段构建：前端（Node）→ 后端（Go，交叉编译）→ 运行镜像（Alpine）
# 单架构：  docker build -t lanpanel .
# 多架构：  docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t <仓库>/lanpanel --push .

ARG GO_VERSION=1.27
ARG NODE_VERSION=22
# 基础镜像仓库。国内网络可改为加速站，例如 --build-arg BASE_REGISTRY=docker.m.daocloud.io
ARG BASE_REGISTRY=docker.io

# ---- 1. 前端 ----
# 前端产物与架构无关，固定在构建机的原生平台上执行，避免多架构构建时用 QEMU 跑 npm
FROM --platform=$BUILDPLATFORM ${BASE_REGISTRY}/library/node:${NODE_VERSION}-alpine AS web
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --registry=${NPM_REGISTRY} --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 2. 后端 ----
FROM --platform=$BUILDPLATFORM ${BASE_REGISTRY}/library/golang:${GO_VERSION}-alpine AS go
ARG TARGETOS TARGETARCH TARGETVARIANT
ARG VERSION=dev
ARG GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0 GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=web /src/web/dist web/dist
# TARGETVARIANT 形如 v7，GOARM 需要去掉前缀 v
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o /out/lanpanel ./cmd/lanpanel

# ---- 3. 运行镜像 ----
FROM ${BASE_REGISTRY}/library/alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go /out/lanpanel /usr/local/bin/lanpanel
ENV LANPANEL_LISTEN=:3080 \
    LANPANEL_DATA=/data \
    TZ=Asia/Shanghai
VOLUME ["/data"]
EXPOSE 3080
# 设备发现（阶段 2）需要原始套接字做 ARP/ICMP，因此以 root 运行，并在 compose 中只授予 NET_RAW
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${LANPANEL_LISTEN##*:}/api/bootstrap" || exit 1
ENTRYPOINT ["/usr/local/bin/lanpanel"]
