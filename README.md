# SumoLogic-Container-Images

## Current List of Images:

### SumoLogic Image list		
Current images	Suggested UBI base images	Dockerfile
### Setup job:		
public.ecr.aws/sumologic/kubernetes-setup:3.0.0	need Dockerfile	https://github.com/SumoLogic/sumologic-kubernetes-setup/blob/main/Dockerfile
### Kube prometheus stack:		
quay.io/prometheus/node-exporter:v1.0.1	registry.redhat.io/openshift4/ose-prometheus-node-exporter:v4.7 (using node-export 1.0.1, OS="Red Hat Enterprise Linux")	
quay.io/coreos/kube-state-metrics:v1.9.7	registry.redhat.io/openshift4/ose-kube-state-metrics:v4.7 (using kube-state-metrics:v1.9.7, OS="Red Hat Enterprise Linux 8.4 (Ootpa)")	
quay.io/prometheus-operator/prometheus-operator:v0.43.2	registry.redhat.io/openshift4/ose-prometheus-operator:v4.7 (using prometheus-operator:v0.44.1, OS="Red Hat Enterprise Linux 8.4 (Ootpa)")	
quay.io/prometheus/prometheus:v2.22.1	registry.redhat.io/openshift4/ose-prometheus:v4.6 (using prometheus version 2.22.2, PRETTY_NAME="Red Hat Enterprise Linux 8.2 (Ootpa)")	
quay.io/prometheus-operator/prometheus-config-reloader:v0.43.2	registry.redhat.io/openshift4/ose-prometheus-config-reloader:v4.7.0 (using prometheus-config-reloader:v0.44.1, OS="Red Hat Enterprise Linux 8.4 (Ootpa)")	
quay.io/thanos/thanos:v0.10.0	registry.redhat.io/openshift4/ose-thanos-rhel8:v4.6 (using thanos:v0.15.0, OS="Red Hat Enterprise Linux 8.2 (Ootpa)")	
### Metrics server		
docker.io/bitnami/metrics-server:0.4.2-debian-10-r58	registry.access.redhat.com/openshift3/ose-metrics-server:v3.11 (OS="Red Hat Enterprise Linux Server 7.9 (Maipo)")	
### Tailing Sidecar:		
gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0	registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.7  (OS="Red Hat Enterprise Linux 8.4 (Ootpa)")	
ghcr.io/sumologic/tailing-sidecar-operator:0.3.0	need Dockerfile	https://github.com/SumoLogic/tailing-sidecar/blob/main/operator/Dockerfile
ghcr.io/sumologic/tailing-sidecar:0.3.0	need Dockerfile	https://github.com/SumoLogic/tailing-sidecar/blob/main/sidecar/Dockerfile
### Telegraf		
quay.io/influxdb/telegraf-operator:v1.1.1	need Dockerfile	
public.ecr.aws/sumologic/telegraf:1.14.4	need Dockerfile	
### Fluentd		
public.ecr.aws/sumologic/kubernetes-fluentd:1.12.2-sumo-0	need Dockerfile but can use registry.redhat.io/openshift4/ose-logging-fluentd:latest	https://github.com/SumoLogic/sumologic-kubernetes-fluentd/blob/main/Dockerfile
### Fluent Bit		
public.ecr.aws/sumologic/fluent-bit:1.6.10	need Dockerfile	https://github.com/fluent/fluent-bit-docker-image
### Falco		
public.ecr.aws/sumologic/falco:0.27.0	need Dockerfile	https://github.com/falcosecurity/falco/blob/master/docker/falco/Dockerfile
busybox:1.33.0 (init container for falco)	need Dockerfile	
### OpenTelemetry		
public.ecr.aws/sumologic/opentelemetry-collector:0.22.0-sumo	need Dockerfile	https://github.com/SumoLogic/opentelemetry-collector-contrib/blob/main/cmd/otelcontribcol/Dockerfile