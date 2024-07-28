FROM docker.io/library/golang:1.21 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY go-huawei-client/ /build/go-huawei-client/

RUN go mod download

COPY cmd/main.go /build/cmd/main.go
COPY pkg/ /build/pkg/
COPY pkg/ /build/pkg/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o eg8145v5-ingress-operator cmd/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /build/eg8145v5-ingress-operator .
USER 65532:65532

ENTRYPOINT ["/eg8145v5-ingress-operator"]
