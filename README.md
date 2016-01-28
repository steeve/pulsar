Quasar daemon
======

Fork of the great [Pulsar daemon](https://github.com/steeve/pulsar)

1. Build the [cross-compiler](https://github.com/scakemyer/cross-compiler) and [libtorrent-go](https://github.com/scakemyer/libtorrent-go)

2. Set GOPATH

    ```
    export GOPATH="~/go"
    ```

3. go get

    ```
    go get github.com/scakemyer/quasar
    ```

    For Windows support, but required for all builds, you also need:

    ```
    go get github.com/mattn/go-isatty
    ```

4. Build environments:

    ```
    make envs
    ```

5. Make specific platforms, or all of them:

    Linux-x64

    ```
    make build TARGET_OS=linux TARGET_ARCH=x64 MARGS="dist"
    ```

    Darwin-x64

    ```
    make build TARGET_OS=darwin TARGET_ARCH=x64 MARGS="dist"
    ```

    Windows

    ```
    make build TARGET_OS=windows TARGET_ARCH=x86 MARGS="dist"
    ```

    All platforms

    ```
    make all
    ```
