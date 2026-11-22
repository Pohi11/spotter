{{/* Names, labels, and the environment shared by several workloads. */}}

{{- define "spotter.name" -}}
{{ .root.Release.Name }}-{{ .component }}
{{- end }}

{{- define "spotter.labels" -}}
app.kubernetes.io/name: {{ .component }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/part-of: spotter
app.kubernetes.io/version: {{ .root.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
helm.sh/chart: {{ .root.Chart.Name }}-{{ .root.Chart.Version }}
{{- end }}

{{- define "spotter.selector" -}}
app.kubernetes.io/name: {{ .component }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- end }}

{{- define "spotter.secretName" -}}
{{ .Values.auth.existingSecret | default (printf "%s-credentials" .Release.Name) }}
{{- end }}

{{- define "spotter.goImage" -}}
{{ .Values.images.go.repository }}:{{ .Values.images.go.tag }}
{{- end }}

{{- define "spotter.pythonImage" -}}
{{ .Values.images.python.repository }}:{{ .Values.images.python.tag }}
{{- end }}

{{- define "spotter.natsHost" -}}
{{ .Release.Name }}-nats
{{- end }}

{{- define "spotter.natsURL" -}}
nats://{{ include "spotter.natsHost" . }}:4222
{{- end }}

{{- define "spotter.s3Host" -}}
{{ .Release.Name }}-seaweedfs
{{- end }}

{{- define "spotter.registryAddr" -}}
{{ .Release.Name }}-registry:9090
{{- end }}

{{- define "spotter.postgresHost" -}}
{{ .Release.Name }}-postgres
{{- end }}

{{/* Whole cores in a CPU quantity ("2", 2, "1500m"), at least 1. */}}
{{- define "spotter.wholeCores" -}}
{{- $c := toString . -}}
{{- $n := 0 -}}
{{- if hasSuffix "m" $c -}}
{{- $n = div (atoi (trimSuffix "m" $c)) 1000 -}}
{{- else -}}
{{- $n = int (float64 $c) -}}
{{- end -}}
{{- max $n 1 -}}
{{- end }}

{{/* S3 settings and credentials (from the Secret only). */}}
{{- define "spotter.s3Env" -}}
- name: S3_ENDPOINT
  value: http://{{ include "spotter.s3Host" . }}:8333
- name: S3_REGION
  value: {{ .Values.s3.region | quote }}
- name: S3_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "spotter.secretName" . }}
      key: s3-access-key
- name: S3_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "spotter.secretName" . }}
      key: s3-secret-key
{{- end }}

{{- define "spotter.otelEnv" -}}
{{- if .root.Values.otelEndpoint }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ .root.Values.otelEndpoint | quote }}
{{- end }}
- name: OTEL_SERVICE_NAME
  value: {{ .service | quote }}
{{- end }}

{{/* Go service settings shared by registry, gateway, and monitor. */}}
{{- define "spotter.goServiceEnv" -}}
- name: SHUTDOWN_TIMEOUT
  value: "{{ .Values.shutdownTimeoutSeconds }}s"
- name: NATS_URL
  value: {{ include "spotter.natsURL" . }}
{{- end }}

{{- define "spotter.terminationGrace" -}}
{{ add .Values.shutdownTimeoutSeconds .Values.shutdownMarginSeconds }}
{{- end }}

{{/* An init container that waits until each host:port accepts TCP connections. */}}
{{- define "spotter.waitFor" -}}
- name: wait-for-dependencies
  image: {{ include "spotter.goImage" .root }}
  imagePullPolicy: {{ .root.Values.images.pullPolicy }}
  command: ["spotterctl", "wait", "-timeout", "10m"{{ range .addrs }}, {{ . | quote }}{{ end }}]
  securityContext:
    {{- include "spotter.restrictedContainer" . | nindent 4 }}
  resources:
    requests: { cpu: 10m, memory: 16Mi }
    limits: { cpu: 100m, memory: 64Mi }
{{- end }}

{{- define "spotter.restrictedPod" -}}
runAsNonRoot: true
seccompProfile:
  type: RuntimeDefault
{{- end }}

{{- define "spotter.restrictedContainer" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: ["ALL"]
{{- end }}

{{/* Liveness and readiness for a Go service on its "http" port. */}}
{{- define "spotter.goProbes" -}}
livenessProbe:
  httpGet: { path: /livez, port: http }
  periodSeconds: 10
  failureThreshold: 3
readinessProbe:
  httpGet: { path: /readyz, port: http }
  periodSeconds: 5
  failureThreshold: 2
{{- end }}
