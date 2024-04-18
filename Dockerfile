FROM alpine:3.3

ENV GOPATH=/

RUN apk add --update ca-certificates go ffmpeg make build-base opus-dev python aria2
RUN apk upgrade

RUN wget https://yt-dl.org/downloads/latest/youtube-dl -O /bin/youtube-dl && chmod a+x /bin/youtube-dl

COPY . /src/github.com/ejv2/mumbledj
COPY config.yaml /root/.config/mumbledj/config.yaml

WORKDIR /src/github.com/ejv2/mumbledj

RUN make
RUN make install
RUN apk del go make build-base && rm -rf /var/cache/apk/*

ENTRYPOINT ["/usr/local/bin/mumbledj"]
