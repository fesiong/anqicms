# supports linux, macOS, freebsd
#
# OEM 品牌标识，对应 config/oem/<OEM>/brand.json。默认 anqicms（官方版）。
# 品牌在编译期通过 -X 固化进二进制，所以同一份源码可产出多个互不影响的安装包，
# 不需要为每个客户维护 fork，也不需要任何运行时开关。
OEM ?= anqicms
# 主程序名必须与 brand.json 的 binary 字段一致：升级逻辑靠它识别压缩包里哪个文件
# 是主程序。官方版是 anqishop（与 start.sh / stop.sh / windowsMake.sh 一致）。
BINARY_NAME ?= anqishop
BIN_SUFFIX :=
GO := go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
LDFLAGS := -ldflags '-w -s -X kandaoni.com/anqicms/config.OEMID=$(OEM)'
ifeq ($(OEM),anqicms)
	ARCHIVE_NAME := $(BINARY_NAME)
else
	ARCHIVE_NAME := $(OEM)
endif
# freebsd needs the noasm tag to avoid sonic/loader native code via cloudwego/base64x
EXTRA_TAGS :=
# ifeq ($(GOOS),freebsd)
# 	EXTRA_TAGS := -tags noasm
# endif
ifeq ($(GOOS),windows)
	BIN_SUFFIX := .exe
endif
ifeq ($(version),)
	version := $(shell git describe --tags --always --dirty="-dev")
endif

.PHONY: all clean tidy oem oem-list brand-check admin build archive

all: clean tidy build archive

# 构建前门禁：品牌数据必须完整，且前端注入文件必须与所选 OEM 一致。
# 放在 build 之前而不是让人记住，是因为“忘了跑”的后果是交付一个品牌错乱的包，
# 而这种错误在界面上只表现为某一处 logo 对不上，验收时极易漏过。
oem:
	@echo "🎨 OEM=$(OEM)"
	@$(GO) run ./cmd/oemgen -check
	@$(GO) run ./cmd/oemgen -oem $(OEM)

oem-list:
	@$(GO) run ./cmd/oemgen -list

# 品牌验收：抓运行中站点的真实响应，检查有没有别的品牌漏出来。
# 要对着 make build 产出的包做，别对着源码起的 dev 服务：dev 用的是仓库里的
# template/default，里面本来就有官方反链（打包时由 -brand-dir 替换），会误报。
brand-check:
	@test -n "$(URL)" || { echo "用法：make brand-check URL=http://127.0.0.1:8001 [OEM=acme]"; exit 1; }
	@$(GO) run ./cmd/oemgen -verify $(URL) -oem $(OEM)

# 后台 SPA 由后端从磁盘 system/ 目录直接服务（route/manage.go），而 system/
# 此前只能靠人手工从 dist 拷贝。这对 OEM 是致命的：二进制是 A 品牌、UI 还是
# 官方版，界面照常工作，只有客户截图时才会发现。所以让构建自己完成这一步。
# 只想验证后端改动时用 SKIP_ADMIN=1 跳过（约省 5 分钟）。
admin: oem
	@echo "🎨 Building admin for OEM=$(OEM)..."
	cd ./anqicms-admin && yarn build
	@echo "📁 Sync dist -> system"
	rm -rf ./system
	mkdir -p ./system
	cp -R ./anqicms-admin/dist/. ./system/

ifeq ($(SKIP_ADMIN),1)
BUILD_DEPS := oem
else
BUILD_DEPS := admin
endif

clean:
	@echo "🧹 Cleaning..."
	@rm -rf ./release
ifeq ($(GOOS),windows)
	cp -r ./source/anqicms_syso ./anqicms.syso
else
	rm -rf ./anqicms.syso
endif

tidy:
	@echo "🧼 Tidying up dependencies..."
	$(GO) mod tidy
	$(GO) mod vendor

build: $(BUILD_DEPS)
	@echo "🔨 Building for $(GOOS)/$(GOARCH)..."
	mkdir -p -v ./release/$(GOOS)/cache
	mkdir -p -v ./release/$(GOOS)/public/static
	mkdir -p -v ./release/$(GOOS)/template
	mkdir -p -v ./release/$(GOOS)/source
	cp -r ./public/static/default ./release/$(GOOS)/public/static/
	cp -r ./public/*.xsl ./release/$(GOOS)/public/ 2>/dev/null || true
	cp -r ./template/default ./release/$(GOOS)/template/
	@$(GO) run ./cmd/oemgen -oem $(OEM) -brand-dir ./release/$(GOOS)/template
	-cp -r ./system ./release/$(GOOS)/
	cp -r ./locales ./release/$(GOOS)/
	cp -r ./CHANGELOG.md ./release/$(GOOS)/
	cp -r ./License ./release/$(GOOS)/
	cp -r ./clientFiles ./release/$(GOOS)/
	cp -r ./README.md ./release/$(GOOS)/
	cp -r ./dictionary.txt ./release/$(GOOS)/
	-cp -r ./source/cwebp_$(GOOS)_$(GOARCH)$(BIN_SUFFIX) ./release/$(GOOS)/source/
	find ./release/$(GOOS) -name '.DS_Store' | xargs rm -f
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath $(EXTRA_TAGS) $(LDFLAGS) -o ./release/$(GOOS)/$(BINARY_NAME)$(BIN_SUFFIX) kandaoni.com/anqicms/main
	strip ./release/$(GOOS)/$(BINARY_NAME)$(BIN_SUFFIX)
	rm -rf anqicms.syso
	@if [ "$(OEM)" != "anqicms" ]; then \
		echo "⚠️  前端品牌文件已切换为 $(OEM)：提交前请执行 git checkout -- anqicms-admin/src/oem"; \
	fi

archive:
	@echo "📦 Creating archive..."
	@(cd ./release/$(GOOS)/ && zip -r -9 ../$(ARCHIVE_NAME)-$(GOOS)-$(GOARCH)-$(version).zip .)