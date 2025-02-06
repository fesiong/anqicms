# only for linux and macos
BINARY_NAME := anqicms
GO := go
GOOS := $(shell $(GO) env GOOS)
ifeq ($(version),)
	version := $(shell git describe --tags --always --dirty="-dev")
endif
LDFLAGS := -ldflags '-w -s'

.PHONY: all clean tidy build archive

all: clean tidy build archive

clean:
	@echo "🧹 Cleaning..."
	@rm -rf ./release
	@rm -rf ./anqicms.syso

tiny:
	@echo "🧼 Tidying up dependencies..."
	$(GO) mod tidy
	$(GO) mod vendor

build:
	@echo "🔨 Building for current platform..."
	mkdir -p -v ./release/$(GOOS)/cache
	mkdir -p -v ./release/$(GOOS)/public
	cp -r ./doc ./release/$(GOOS)/
	cp -r ./public/static ./release/$(GOOS)/public/
	cp -r ./public/*.xsl ./release/$(GOOS)/public/
	cp -r ./template ./release/$(GOOS)/
	cp -r ./system ./release/$(GOOS)/
	cp -r ./locales ./release/$(GOOS)/
	cp -r ./CHANGELOG.md ./release/$(GOOS)/
	cp -r ./License ./release/$(GOOS)/
	cp -r ./clientFiles ./release/$(GOOS)/
	cp -r ./README.md ./release/$(GOOS)/
	cp -r ./dictionary.txt ./release/$(GOOS)/
	find ./release/$(GOOS) -name '.DS_Store' | xargs rm -f
	$(GO) build -trimpath $(LDFLAGS) -o ./release/$(GOOS)/$(BINARY_NAME) kandaoni.com/anqicms/main

archive:
	@echo "📦 Creating archive..."
	@(cd ./release/$(GOOS)/ && zip -r -9 ../$(BINARY_NAME)-$(GOOS)-$(version).zip .)