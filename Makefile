CC = cc
CXX = c++
STRIP = strip

include platform_host.mk

ifneq ($(CROSS_TRIPLE),)
	CC := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-$(CC)
	CXX := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-$(CXX)
	STRIP := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-strip
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
	GO_LDFLAGS = -extld=$(CC)
else ifeq ($(TARGET_OS), darwin)
	EXT =
	GOOS = darwin
	# Needs this or cgo will try to link with libgcc, which will fail
	CC := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang
	CXX := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang++
	GO_LDFLAGS = -linkmode=external -extld=$(CC)
else ifeq ($(TARGET_OS), linux)
	EXT =
	GOOS = linux
	GO_LDFLAGS = -linkmode=external -extld=$(CC)
else ifeq ($(TARGET_OS), android)
	EXT =
	GOOS = linux
	GOARM = 7
	GO_LDFLAGS = -linkmode=external -extld=$(CC)
endif

NAME = pulsar
GO_PKG = github.com/steeve/pulsar
GO = go
GIT = git
DOCKER = docker
UPX = upx
GIT_VERSION = $(shell $(GIT) describe --always)
VERSION = $(shell cat VERSION)
#VERSION = $(patsubst v%,%,$(GIT_VERSION))
ZIP_FILE = $(ADDON_NAME)-$(VERSION).zip
CGO_ENABLED = 1
OUTPUT_NAME = $(NAME)$(EXT)
BUILD_PATH = build/$(TARGET_OS)_$(TARGET_ARCH)
LIBTORRENT_GO = github.com/steeve/libtorrent-go
LIBTORRENT_GO_HOME = $(shell go env GOPATH)/src/$(LIBTORRENT_GO)
GO_BUILD_TAGS = netgo
GO_LDFLAGS += -w -X $(GO_PKG)/util.Version "$(VERSION)" -X $(GO_PKG)/util.GitCommit "$(GIT_VERSION)"

force:
	@true

libtorrent-go: force
	$(MAKE) -C $(LIBTORRENT_GO_HOME) clean all

$(BUILD_PATH):
	mkdir -p $(BUILD_PATH)

$(BUILD_PATH)/$(OUTPUT_NAME): $(BUILD_PATH)
	CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -v -tags $(GO_BUILD_TAGS) -o $(BUILD_PATH)/$(OUTPUT_NAME) -ldflags="$(GO_LDFLAGS)"

vendor_libs_windows:
	cp -f $(shell go env GOPATH)/src/github.com/steeve/libtorrent-go/$(BUILD_PATH)/* $(BUILD_PATH)

pulsar: $(BUILD_PATH)/$(OUTPUT_NAME)

clean:
	rm -rf $(BUILD_PATH)

distclean:
	rm -rf build

build: force
	$(DOCKER) run -i --rm -v $(HOME):$(HOME) -t -e GOPATH=$(shell go env GOPATH) -w $(shell pwd) steeve/pulsar-$(TARGET_OS)-$(TARGET_ARCH) make $(MARGS) GIT_VERSION=$(GIT_VERSION)

strip: force
	$(STRIP) $(BUILD_PATH)/$(OUTPUT_NAME)

upx: force
	@find build/ -type f -exec $(UPX) {} \;

dist: pulsar strip upx

alldist: force
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=x64 MARGS=pulsar
