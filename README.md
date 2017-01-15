Quasar daemon [![Build Status](https://travis-ci.org/scakemyer/quasar.svg?branch=master)](https://travis-ci.org/scakemyer/quasar)
======

Fork of the great [Pulsar daemon](https://github.com/steeve/pulsar)

1. Build the [cross-compiler](https://github.com/scakemyer/cross-compiler) and [libtorrent-go](https://github.com/scakemyer/libtorrent-go) images,
    or alternatively, pull the libtorrent-go images from [Docker Hub](https://hub.docker.com/r/quasarhq/libtorrent-go):

    ```
    make pull-all
    ```

    Or for a specific platform:
    ```
    make pull PLATFORM=android-x64
    ```

2. Set GOPATH

    ```
    export GOPATH="~/go"
    ```

3. go get

    ```
    go get -d github.com/scakemyer/quasar
    ```

    For Windows support, but required for all builds, you also need:

    ```
    go get github.com/mattn/go-isatty
    ```

4. Build libtorrent-go libraries:

    ```
    make libs
    ```

5. Make specific platforms, or all of them:

    Linux-x64
    ```
    make linux-x64
    ```

    Darwin-x64
    ```
    make darwin-x64
    ```

    Windows
    ```
    make windows-x86
    ```

    All platforms
    ```
    make
    ```
