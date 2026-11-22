# All Go binaries in one image (decision record 0014); each workload picks its command.
FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ \
      ./services/registry/cmd/registry \
      ./services/gateway/cmd/gateway \
      ./services/monitor/cmd/monitor \
      ./cmd/spotterctl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
# Distroless "nonroot" by number, so Kubernetes runAsNonRoot can verify it.
USER 65532:65532
ENTRYPOINT []
