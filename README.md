pulsar
======
##### Compilar:
1. Install Docker, Golang e mercurial

2. Set GOPATH

    ``` 
    export GOPATH="/home/user/Go"
    ```
    
3. go gets

    ```
    go get github.com/op/go-logging
    ```
    
    ```
    go get github.com/i96751414/pulsar
    ```
    
    ```
    go get github.com/mattn/go-isatty
    ```

4. Build environments:

    Once I could not upload the images to hub, you should do this for all dependencies, that is, cross-compiler and libtorrent-go. Be aware that this will take time.
    
    ```
    make build-envs
    ```
    
5. Make (Examples):

    ```
    make build TARGET_OS=linux TARGET_ARCH=x64 MARGS="dist"
    ```
    
    ```
    make build TARGET_OS=darwin TARGET_ARCH=x64 MARGS="dist"
    ```
    
    ```
    make build TARGET_OS=windows TARGET_ARCH=x86 MARGS="dist"
    ```

##### Note - MARGS:

You should compile libtorrent-go at least one time. For that, just set MARGS="libtorrent-go"

```bash
MARGS="libtorrent-go"   # libtorrent
MARGS="dist"            # Pulsar
```
