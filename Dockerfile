# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.20 as builder

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY cache/ cache/
COPY controllers/ controllers/
COPY gitops/ gitops/
COPY loader/ loader/
COPY metadata/ metadata/
COPY metrics/ metrics/
COPY syncer/ syncer/
COPY tekton/ tekton/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

ARG ENABLE_WEBHOOKS=true
ENV ENABLE_WEBHOOKS=${ENABLE_WEBHOOKS}

# Use ubi-micro as minimal base image to package the manager binary
# See https://catalog.redhat.com/software/containers/ubi9/ubi-micro/615bdf943f6014fa45ae1b58
FROM registry.access.redhat.com/ubi9/ubi-micro:9.2-15.1696515526
COPY --from=builder /opt/app-root/src/manager /

# It is mandatory to set these labels
LABEL description="RHTAP Release Service"
LABEL io.k8s.description="RHTAP Release Service"
LABEL io.k8s.display-name="release-service"
LABEL summary="RHTAP Release Service"

USER 65532:65532

ENTRYPOINT ["/manager"]
