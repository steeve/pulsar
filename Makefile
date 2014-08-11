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
else ifeq ($(TARGET_OS), darwin)
	EXT =
	GOOS = darwin
	# Needs this or cgo will try to link with libgcc, which will fail
	CC := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang
	CXX := $(CROSS_ROOT)/bin/$(CROSS_TRIPLE)-clang++
else ifeq ($(TARGET_OS), linux)
	EXT =
	GOOS = linux
else ifeq ($(TARGET_OS), android)
	EXT =
	GOOS = linux
	GOARM = 7
endif

NAME = pulsar
GO = go
GIT = git
GIT_VERSION = $(shell $(GIT) describe --always)
VERSION = $(patsubst v%,%,$(GIT_VERSION))
ZIP_FILE = $(ADDON_NAME)-$(VERSION).zip
CGO_ENABLED = 1
OUTPUT_NAME = $(NAME)$(EXT)
BUILD_PATH = build/$(TARGET_OS)_$(TARGET_ARCH)
LIBTORRENT_GO = github.com/steeve/libtorrent-go
LIBTORRENT_GO_HOME = $(shell go env GOPATH)/src/$(LIBTORRENT_GO)


all: clean libtorrent-go dist

force:
	true

test:
	@echo $(CC) $(CROSS_TRIPLE)

libtorrent-go: force
	$(MAKE) -C $(LIBTORRENT_GO_HOME) clean all

run: build
ifeq ($(OS),Linux)
	LD_LIBRARY_PATH=$(BUILD_PATH):$$LD_LIBRARY_PATH $(BUILD_PATH)/$(OUTPUT_NAME)
endif
ifeq ($(OS),Darwin)
	DYLD_LIBRARY_PATH=$(BUILD_PATH):$$DYLD_LIBRARY_PATH $(BUILD_PATH)/$(OUTPUT_NAME)
endif


$(BUILD_PATH):
	mkdir -p $(BUILD_PATH)

$(BUILD_PATH)/$(OUTPUT_NAME): $(BUILD_PATH)
ifeq ($(TARGET_OS), windows)
	CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -v -o $(BUILD_PATH)/$(OUTPUT_NAME) -ldflags="-extld=$(CC)"
else
	CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -v -o $(BUILD_PATH)/$(OUTPUT_NAME) -ldflags="-linkmode=external -extld=$(CC)"
endif

vendor_libs_darwin:

vendor_libs_android:

vendor_libs_linux:

vendor_libs_windows:
	cp -f $(shell go env GOPATH)/src/github.com/steeve/libtorrent-go/$(BUILD_PATH)/* $(BUILD_PATH)

dist: $(BUILD_PATH)/$(OUTPUT_NAME) vendor_libs_$(TARGET_OS)

clean:
	rm -rf $(BUILD_PATH)

distclean:
	rm -rf build


darwin: force
	$(MAKE) clean all

linux32: force
	$(MAKE) clean all TARGET_OS=linux CROSS_TRIPLE=i586-pc-linux CROSS_ROOT=/usr/local/gcc-4.8.1-for-linux32

linux64: force
	$(MAKE) clean all TARGET_OS=linux CROSS_TRIPLE=x86_64-pc-linux CROSS_ROOT=/usr/local/gcc-4.8.0-linux64

linux-rpi: force
	$(MAKE) clean all TARGET_OS=linux ARCH=arm CROSS_TRIPLE=arm-linux-gnueabihf CROSS_ROOT=/usr/local/gcc-linaro-arm-linux-gnueabihf-raspbian

android: force
	$(MAKE) clean all TARGET_OS=android ARCH=arm CROSS_TRIPLE=arm-linux-androideabi CROSS_ROOT=/usr/local/gcc-4.8.0-arm-linux-androideabi

windows: force
	$(MAKE) clean all TARGET_OS=windows CROSS_TRIPLE=i586-mingw32 CROSS_ROOT=/usr/local/gcc-4.8.0-mingw32
