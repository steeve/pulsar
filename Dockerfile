FROM steeve/libtorrent-go:TAG
MAINTAINER Steeve Morin "steeve.morin@gmail.com"

RUN apt-get update && apt-get -y --force-yes install upx-ucl
