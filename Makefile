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
	ifeq ($(TARGET_ARCH), arm)
		GOARM = 7
	else
		GOARM =
	endif
	GO_LDFLAGS = -linkmode=external -extldflags=-pie -extld=$(CC)
endif

NAME = quasar
GO_PKG = github.com/scakemyer/quasar
GO = go
GIT = git
DOCKER = docker
DOCKER_IMAGE = quasar
UPX = upx
GIT_VERSION = $(shell $(GIT) describe --tags)
VERSION = $(shell cat VERSION)
ZIP_FILE = $(ADDON_NAME)-$(VERSION).zip
CGO_ENABLED = 1
OUTPUT_NAME = $(NAME)$(EXT)
BUILD_PATH = build/$(TARGET_OS)_$(TARGET_ARCH)
LIBTORRENT_GO = github.com/scakemyer/libtorrent-go
LIBTORRENT_GO_HOME = $(shell go env GOPATH)/src/$(LIBTORRENT_GO)
GO_BUILD_TAGS =
GO_LDFLAGS += -w -X $(GO_PKG)/util.Version="$(VERSION)" -X $(GO_PKG)/util.GitCommit="$(GIT_VERSION)"
PLATFORMS = \
	darwin-x64 \
	windows-x86 \
	windows-x64 \
	linux-x86 \
	linux-x64 \
	linux-arm \
	android-arm \
	android-x64

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

quasar: $(BUILD_PATH)/$(OUTPUT_NAME)

clean:
	rm -rf $(BUILD_PATH)

distclean:
	rm -rf build

env:
	cat Dockerfile | sed -e s/TAG/$(PLATFORM)/ | $(DOCKER) build -t $(DOCKER_IMAGE):$(PLATFORM) -

envs:
	for i in $(PLATFORMS); do \
		$(MAKE) env PLATFORM=$$i;\
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
dist: quasar vendor_$(TARGET_OS) strip checksum
else
dist: quasar vendor_$(TARGET_OS) strip upx checksum
endif

all: force
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x86 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm MARGS="dist"
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=x86 MARGS="dist"
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=x64 MARGS="dist"
	$(MAKE) build TARGET_OS=android TARGET_ARCH=arm MARGS="dist"
	$(MAKE) build TARGET_OS=android TARGET_ARCH=x64 MARGS="dist"

libs: force
	$(MAKE) libtorrent-go

binaries:
	git config user.name "Travis CI binaries"
	git config user.email "quasar@noreply.email"
	git config --global push.default simple
	git clone --depth=1 https://github.com/scakemyer/quasar-binaries binaries
	cp -Rf build/* binaries/
	cd binaries && git add * && git commit -m "Update to ${GIT_VERSION}"

pull:
	for i in $(PLATFORMS); do \
		docker pull quasarhq/libtorrent-go:$$i; \
	done

envs-from-pull:
	for i in $(PLATFORMS); do \
		docker tag quasarhq/libtorrent-go:$$i libtorrent-go:$$i; \
		$(MAKE) env PLATFORM=$$i; \
	done

push:
	for i in $(PLATFORMS); do \
	  docker tag quasar:$$i quasarhq/quasar:$$i; \
	  docker push quasarhq/quasar:$$i; \
	done
