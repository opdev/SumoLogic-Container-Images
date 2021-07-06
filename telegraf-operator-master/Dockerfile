FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go sidecar.go handler.go class_data.go errors.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager *.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]



#Testing steps
#   helm upgrade --install test-openshift sumologic/sumologic \
#   --set sumologic.accessKey="9VB8bWJH9jWFbwqoEy0b6NPOsLiBbD0ifYcvvrH4TXE1DXgfHv2feIYuuWIaFpYY" \
#   --set sumologic.endpoint="http://receiver-mock.receiver-mock:3000/terraform/api/" \
#   --set sumologic.scc.create=true \
#   --set fluent-bit.securityContext.privileged=true \
#   --set kube-prometheus-stack.prometheus-node-exporter.service.port=9200 \
#   --set kube-prometheus-stack.prometheus-node-exporter.service.targetPort=9200 \
#   --set fluentd.buffer.queueChunkLimitSize=256 \
#   --set fluentd.buffer.totalLimitSize="256m" \
#   --set fluentd.logs.containers.multiline.enabled=false \
#   --set metrics-server.enabled=true \
#   --set metrics-server.apiService.create=false \
#   --set otelagent.enabled=true \
#   --set telegraf-operator.enabled=true \
#   --set telegraf-operator.image.repository=quay.io/yoza/telegraf-operator-ubi-test \
#   --set falco.enabled=false \
#   --set tailing-sidecar-operator.enabled=true \
#   --version 2.1.1 \
#   -n sumologic