# syntax=docker/dockerfile:1.6
FROM golang:1 AS builder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go install github.com/silvercory/slog-schema/cmd/protoc-gen-slogschema@latest

# When building a Docker image on a host that doesn't match linux/amd64 (such as an M1),
# go install will put the binary in $GOPATH/bin/$GOOS_$GOARCH/. The mv command copies
# the binary to /go/bin so subsequent steps don't fail when copying from the builder.
RUN mv /go/bin/linux_amd64/* /go/bin || true

FROM scratch
COPY --from=builder --link /etc/passwd /etc/passwd
COPY --from=builder /go/bin/ /
USER nobody
ENTRYPOINT [ "/protoc-gen-slogschema" ]