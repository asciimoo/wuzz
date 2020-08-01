FROM alpine:3.12 AS permissions-giver

# Make sure docker-entrypoint.sh is executable, regardless of the build host.
WORKDIR /out
COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh

FROM golang:1.14-alpine3.12 AS builder

# Build wuzz
WORKDIR /out
COPY . .
RUN go build .

FROM alpine:3.12 AS organizer

# Prepare executables
WORKDIR /out
COPY --from=builder /out/wuzz .
COPY --from=permissions-giver /out/docker-entrypoint.sh .

FROM alpine:3.12 AS runner
WORKDIR /wuzz
COPY --from=organizer /out /usr/local/bin
ENTRYPOINT [ "docker-entrypoint.sh" ]