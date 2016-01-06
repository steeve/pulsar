pulsar
======
##### Compilar:
1. Install Docker, Golang e mercurial

2. Set o GOPATH

    ``` 
    export GOPATH="/home/user/Go"
    ```
    
3. Fazer go gets

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

    ```
    make build-envs
    ```
    
5. Make (Examples):

    ```
    make build TARGET_OS=linux TARGET_ARCH=x64 PKGCP=/usr/x86_64-linux-gnu/lib/pkgconfig MARGS="dist"
    ```
    
    ```
    make build TARGET_OS=darwin TARGET_ARCH=x64 PKGCP=/usr/x86_64-apple-darwin14/lib/pkgconfig MARGS="dist"
    ```
    
    ```
    make build TARGET_OS=windows TARGET_ARCH=x86 PKGCP=/usr/x86_64-w64-mingw32/lib/pkgconfig MARGS="dist"
    ```

##### Note - MARGS:

You should compile libtorrent-go at least one time. For that, just set MARGS="libtorrent-go"

```bash
MARGS="libtorrent-go"   # libtorrent
MARGS="dist"            # Pulsar
```
