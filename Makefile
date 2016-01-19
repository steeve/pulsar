CC = cc
CXX = c++
STRIP = strip

include platform_host.mk

ifneq ($(CROSS_TRIPLE),)
	CC := $(CROSS_TRIPLE)-$(CC)
	CXX := $(CROSS_TRIPLE)-$(CXX)
	STRIP := $(CROSS_TRIPLE)-strip
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
	GOOS = android
	GOARM = 7
	GO_LDFLAGS = -linkmode=external -extldflags=-pie -extld=$(CC)
endif

NAME = pulsar
GO_PKG = github.com/i96751414/pulsar
GO = go
GIT = git
DOCKER = docker
DOCKER_IMAGE = pulsar
UPX = upx
GIT_VERSION = $(shell $(GIT) describe --always)
VERSION = $(shell cat VERSION)
ZIP_FILE = $(ADDON_NAME)-$(VERSION).zip
CGO_ENABLED = 1
OUTPUT_NAME = $(NAME)$(EXT)
BUILD_PATH = build/$(TARGET_OS)_$(TARGET_ARCH)
LIBTORRENT_GO = github.com/i96751414/libtorrent-go
LIBTORRENT_GO_HOME = $(shell go env GOPATH)/src/$(LIBTORRENT_GO)
GO_BUILD_TAGS =
GO_LDFLAGS += -w -X $(GO_PKG)/util.Version "$(VERSION)" -X $(GO_PKG)/util.GitCommit "$(GIT_VERSION)"
PLATFORMS = darwin-x64 windows-x86 windows-x64 linux-x86 linux-x64 linux-arm android-arm

force:
	@true

libtorrent-go: force
	$(MAKE) -C $(LIBTORRENT_GO_HOME)

$(BUILD_PATH):
	mkdir -p $(BUILD_PATH)

$(BUILD_PATH)/$(OUTPUT_NAME): $(BUILD_PATH) force
	LDFLAGS='$(LDFLAGS)' \
	CC='$(CC)' CXX='$(CXX)' \
	GOOS='$(GOOS)' GOARCH='$(GOARCH)' GOARM='$(GOARM)' \
	CGO_ENABLED='$(CGO_ENABLED)' \
	$(GO) build -v \
		-gcflags '$(GO_GCFLAGS)' \
		-ldflags '$(GO_LDFLAGS)' \
		-o '$(BUILD_PATH)/$(OUTPUT_NAME)'

vendor_darwin vendor_linux:

vendor_windows:
	find $(shell go env GOPATH)/pkg/$(GOOS)_$(GOARCH) -name *.dll -exec cp -f {} $(BUILD_PATH) \;

vendor_android:
	cp $(CROSS_ROOT)/$(CROSS_TRIPLE)/lib/libgnustl_shared.so $(BUILD_PATH)


vendor_libs_windows:

vendor_libs_android:
	$(CROSS_ROOT)/arm-linux-androideabi/lib/libgnustl_shared.so

pulsar: $(BUILD_PATH)/$(OUTPUT_NAME)

clean:
	rm -rf $(BUILD_PATH)

distclean:
	rm -rf build

build-envs:
	for i in $(PLATFORMS); do \
		cat Dockerfile | sed -e s/TAG/$$i/ | $(DOCKER) build -t $(DOCKER_IMAGE):$$i - ;\
	done

build: force
	$(DOCKER) run --rm -v $(GOPATH):/go -e GOPATH=/go -v $(shell pwd):/go/src/$(GO_PKG) -w /go/src/$(GO_PKG) $(DOCKER_IMAGE):$(TARGET_OS)-$(TARGET_ARCH) make $(MARGS) TARGET_OS=$(TARGET_OS) TARGET_ARCH=$(TARGET_ARCH) GIT_VERSION=$(GIT_VERSION)

docker: force
	$(DOCKER) run --rm -v $(GOPATH):/go -e GOPATH=/go -v $(shell pwd):/go/src/$(GO_PKG) -w /go/src/$(GO_PKG) $(DOCKER_IMAGE):$(TARGET_OS)-$(TARGET_ARCH)

strip: force
	@find $(BUILD_PATH) -type f ! -name "*.exe" -exec $(STRIP) {} \;

upx: force
# Do not .exe files, as upx doesn't really work with 8l/6l linked files.
# It's fine for other platforms, because we link with an external linker, namely
# GCC or Clang. However, on Windows this feature is not yet supported.
	@find $(BUILD_PATH) -type f ! -name "*.exe" -a ! -name "*.so"  -exec $(UPX) --lzma {} \;

checksum: $(BUILD_PATH)/$(OUTPUT_NAME)
	shasum -b $(BUILD_PATH)/$(OUTPUT_NAME) | cut -d' ' -f1 >> $(BUILD_PATH)/$(OUTPUT_NAME)


ifeq ($(TARGET_ARCH), arm)
dist: pulsar vendor_$(TARGET_OS) strip checksum
else
dist: pulsar vendor_$(TARGET_OS) strip upx checksum
endif

alldist: force
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x86 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm MARGS="dist"
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=x86 MARGS="dist"
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=android TARGET_ARCH=arm MARGS="dist"

libs: force
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=x64 MARGS="libtorrent-go"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x86 MARGS="libtorrent-go"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x64 MARGS="libtorrent-go"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm MARGS="libtorrent-go"
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=x86 MARGS="libtorrent-go"
	$(MAKE) build TARGET_OS=android TARGET_ARCH=arm MARGS="libtorrent-go"
