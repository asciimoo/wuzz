FROM golang:1.7-alpine

RUN apk add --no-cache \
      git

ENV GOPATH $HOME/gocode
ENV PATH $GOPATH/bin:$PATH

COPY docker/entry.sh /docker-entrypoint
RUN chmod u+x /docker-entrypoint

RUN go get github.com/asciimoo/wuzz

ENTRYPOINT ["/docker-entrypoint"]