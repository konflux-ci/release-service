# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:9.7-1771271449 as builder

# Build arguments
ARG ENABLE_COVERAGE=false

USER 1001

# Copy the Go Modules manifests
COPY --chown=1001:0 go.mod go.mod
COPY --chown=1001:0 go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY --chown=1001:0 . .

# Build with or without coverage instrumentation
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then \
        echo "Building with coverage instrumentation..."; \
        CGO_ENABLED=0 go build -cover -covermode=atomic -tags=coverage -o manager .; \
    else \
        echo "Building production binary..."; \
        CGO_ENABLED=0 go build -a -o manager .; \
    fi

ARG ENABLE_WEBHOOKS=true
ENV ENABLE_WEBHOOKS=${ENABLE_WEBHOOKS}

# Use ubi-micro as minimal base image to package the manager binary
# See https://catalog.redhat.com/software/containers/ubi9/ubi-micro/615bdf943f6014fa45ae1b58
FROM registry.access.redhat.com/ubi9/ubi-micro:9.7-1771346390
COPY --from=builder /opt/app-root/src/manager /

# It is mandatory to set these labels
LABEL name="Konflux Release Service"
LABEL description="Konflux Release Service"
LABEL io.k8s.description="Konflux Release Service"
LABEL io.k8s.display-name="release-service"
LABEL summary="Konflux Release Service"
LABEL com.redhat.component="release-service"
LABEL io.openshift.tags="konflux"
LABEL vendor="Red Hat, Inc."
LABEL distribution-scope="public"
LABEL release="1"
LABEL url="github.com/konflux-ci/release-service"
LABEL version="1"

USER 65532:65532

ENTRYPOINT ["/manager"]
