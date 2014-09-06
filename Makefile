CC = gcc
CXX = c++

include platform_host.mk

ifneq ($(CROSS_TRIPLE),)
	CC := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-$(CC)
	CXX := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-$(CXX)
endif

include platform_target.mk

ifeq ($(TARGET_ARCH),x86)
	GOARCH = 386
else ifeq ($(TARGET_ARCH),x64)
	GOARCH = amd64
else ifeq ($(TARGET_ARCH),arm)
	GOARCH = arm
	GOARM = 6
endif

ifeq ($(TARGET_OS), windows)
	EXT = .exe
	GOOS = windows
	LDFLAGS = -extld=$(CC)
else ifeq ($(TARGET_OS), darwin)
	EXT =
	GOOS = darwin
	# Needs this or cgo will try to link with libgcc, which will fail
	CC := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang
	CXX := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang++
	LDFLAGS = -linkmode=external -extld=$(CC)
else ifeq ($(TARGET_OS), linux)
	EXT =
	GOOS = linux
	LDFLAGS = -linkmode=external -extld=$(CC)
else ifeq ($(TARGET_OS), android)
	EXT =
	GOOS = linux
	GOARM = 7
	LDFLAGS = -linkmode=external -extld=$(CC)
endif

NAME = pulsar
GO_PKG = github.com/steeve/pulsar
GO = go
GIT = git
DOCKER = docker
GIT_VERSION = $(shell $(GIT) describe --always)
VERSION = $(patsubst v%,%,$(GIT_VERSION))
ZIP_FILE = $(ADDON_NAME)-$(VERSION).zip
CGO_ENABLED = 1
OUTPUT_NAME = $(NAME)$(EXT)
BUILD_PATH = build/$(TARGET_OS)_$(TARGET_ARCH)
LIBTORRENT_GO = github.com/steeve/libtorrent-go
LIBTORRENT_GO_HOME = $(shell go env GOPATH)/src/$(LIBTORRENT_GO)
GO_BUILD_TAGS = netgo
LDFLAGS += -w -X $(GO_PKG)/util.Version "$(VERSION)" -X $(GO_PKG)/util.GitCommit "$(GIT_VERSION)"

force:
	true

libtorrent-go: force
	$(MAKE) -C $(LIBTORRENT_GO_HOME) clean all

$(BUILD_PATH):
	mkdir -p $(BUILD_PATH)

$(BUILD_PATH)/$(OUTPUT_NAME): $(BUILD_PATH)
	CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -v -tags $(GO_BUILD_TAGS) -o $(BUILD_PATH)/$(OUTPUT_NAME) -ldflags="$(LDFLAGS)"

vendor_libs_windows:
	cp -f $(shell go env GOPATH)/src/github.com/steeve/libtorrent-go/$(BUILD_PATH)/* $(BUILD_PATH)

dist: $(BUILD_PATH)/$(OUTPUT_NAME)

clean:
	rm -rf $(BUILD_PATH)

distclean:
	rm -rf build

darwin-64:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-osx make clean dist TARGET_OS=darwin TARGET_ARCH=x64 GIT_VERSION=$(GIT_VERSION)

windows-32:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-windows-32 make clean dist TARGET_OS=windows TARGET_ARCH=x86

android-arm:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-android make clean dist TARGET_OS=android TARGET_ARCH=arm

linux-32:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-linux-32 make clean dist TARGET_OS=linux TARGET_ARCH=x86

linux-64:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-linux-64 make clean dist TARGET_OS=linux TARGET_ARCH=x64

linux-arm:
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) pulsar-linux-arm make clean dist TARGET_OS=linux TARGET_ARCH=arm

all: darwin-64 windows-32 android-arm linux-32 linux-64 linux-arm
