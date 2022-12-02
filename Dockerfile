ARG ALPINE_VER=3.16
ARG GO_VER=1.19

FROM alpine:$ALPINE_VER AS permissions-giver

# Make sure docker-entrypoint.sh is executable, regardless of the build host.
WORKDIR /out
COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh

FROM golang:$GO_VER-alpine$ALPINE_VER AS builder

# Build wuzz
WORKDIR /out
COPY . .
RUN go mod tidy
RUN go build .

FROM alpine:$ALPINE_VER AS organizer

# Prepare executables
WORKDIR /out
COPY --from=builder /out/wuzz .
COPY --from=permissions-giver /out/docker-entrypoint.sh .

FROM alpine:$ALPINE_VER AS runner
WORKDIR /wuzz
COPY --from=organizer /out /usr/local/bin
ENTRYPOINT [ "docker-entrypoint.sh" ]