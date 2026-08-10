# The development image: build from source, the way CI does. The release image
# is Dockerfile.release, which takes a binary goreleaser has already built for
# the target platform.
FROM golang:1.26-alpine AS build

ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/tamnd/bourbaki-solver.Version=${VERSION}" \
      -o /out/bourbaki ./cmd/bourbaki

FROM alpine:3.22

# poppler is not optional. Reading a born-digital volume is pdftohtml,
# rendering a scan is pdftoppm, and classifying either is pdfinfo and
# pdfimages. Without them the binary starts and can do nothing.
RUN apk add --no-cache ca-certificates poppler-utils \
 && adduser -D -u 10001 bourbaki

COPY --from=build /out/bourbaki /usr/bin/bourbaki

USER bourbaki

ENTRYPOINT ["/usr/bin/bourbaki"]
