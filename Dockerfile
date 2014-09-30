FROM steeve/libtorrent-go:TARGET_OS-TARGET_ARCH
MAINTAINER Steeve Morin "steeve.morin@gmail.com"

RUN apt-get update && apt-get -y --force-yes install upx-ucl
