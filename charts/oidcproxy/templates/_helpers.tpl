{{/*
Expand the name of the chart.
*/}}
{{- define "oidcproxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "oidcproxy.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "oidcproxy.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "oidcproxy.labels" -}}
helm.sh/chart: {{ include "oidcproxy.chart" . }}
{{ include "oidcproxy.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "oidcproxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "oidcproxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "oidcproxy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "oidcproxy.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Short, predictable name for an upstream apiserver host. Uses the second DNS
label, e.g. apiserver.dev-fss.nais.io -> dev-fss.
*/}}
{{- define "oidcproxy.upstreamName" -}}
{{- $labels := splitList "." . -}}
{{- if gt (len $labels) 1 -}}
{{- index $labels 1 -}}
{{- else -}}
{{- index $labels 0 -}}
{{- end -}}
{{- end }}

{{/*
Predictable ingress host for an upstream apiserver host, e.g.
apiserver.dev-fss.nais.io -> dev-fss-apiserver-oidc.external.nav.cloud.nais.io
*/}}
{{- define "oidcproxy.ingressHost" -}}
{{- printf "%s-apiserver-oidc.%s" (include "oidcproxy.upstreamName" .upstream) .domain -}}
{{- end }}
