FROM libtorrent-go:TAG

RUN curl -L http://sourceforge.net/projects/upx/files/upx/3.91/upx-3.91-amd64_linux.tar.bz2/download | tar xvj && \
    cp upx-3.91-amd64_linux/upx /usr/bin/upx && \
    rm -rf upx-3.91-amd64_linux
