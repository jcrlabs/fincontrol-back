{{- define "fincontrol-back.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "fincontrol-back.fullname" -}}
{{- printf "%s" (include "fincontrol-back.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "fincontrol-back.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "fincontrol-back.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "fincontrol-back.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fincontrol-back.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
