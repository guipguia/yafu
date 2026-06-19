# syntax=docker/dockerfile:1

# ---- web build ----
FROM node:26-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# ---- go build ----
# go.mod pins go 1.26+ via the controller-runtime/k8s.io stack; the toolchain
# auto-downloads when the base image is older.
FROM golang:1.26-alpine AS go-build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist /src/internal/web/dist

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -tags embed \
    -ldflags="-s -w \
      -X github.com/guipguia/yafu/internal/version.Version=${VERSION} \
      -X github.com/guipguia/yafu/internal/version.Commit=${COMMIT} \
      -X github.com/guipguia/yafu/internal/version.Date=${DATE}" \
    -o /out/yafu ./cmd/yafu

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build /out/yafu /usr/local/bin/yafu
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/yafu"]
