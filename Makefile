# only for linux and macos
BINARY_NAME := anqicms
BIN_SUFFIX :=
GO := go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
LDFLAGS := -ldflags '-w -s'
ifeq ($(GOOS),windows)
	BIN_SUFFIX := .exe
	LDFLAGS := -ldflags '-w -s -H=windowsgui'
endif
ifeq ($(version),)
	version := $(shell git describe --tags --always --dirty="-dev")
endif

.PHONY: all clean tidy build archive

all: clean tidy build archive

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

build:
	@echo "🔨 Building for current platform..."
	mkdir -p -v ./release/$(GOOS)/cache
	mkdir -p -v ./release/$(GOOS)/public
	mkdir -p -v ./release/$(GOOS)/source
	cp -r ./doc ./release/$(GOOS)/
	cp -r ./public/static ./release/$(GOOS)/public/
	cp -r ./public/*.xsl ./release/$(GOOS)/public/ 2>/dev/null || true
	cp -r ./template ./release/$(GOOS)/
	cp -r ./locales ./release/$(GOOS)/
	cp -r ./CHANGELOG.md ./release/$(GOOS)/
	cp -r ./License ./release/$(GOOS)/
	cp -r ./clientFiles ./release/$(GOOS)/
	cp -r ./README.md ./release/$(GOOS)/
	cp -r ./dictionary.txt ./release/$(GOOS)/
	cp -r ./source/cwebp_$(GOOS)_$(GOARCH)$(BIN_SUFFIX) ./release/$(GOOS)/source/
	find ./release/$(GOOS) -name '.DS_Store' | xargs rm -f
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath $(LDFLAGS) -o ./release/$(GOOS)/$(BINARY_NAME)$(BIN_SUFFIX) kandaoni.com/anqicms/main
	strip ./release/$(GOOS)/$(BINARY_NAME)$(BIN_SUFFIX)
	rm -rf anqicms.syso

archive:
	@echo "📦 Creating archive..."
	@(cd ./release/$(GOOS)/ && zip -r -9 ../$(BINARY_NAME)-$(GOOS)-$(GOARCH)-$(version).zip .)