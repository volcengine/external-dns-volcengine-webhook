{{/*
Expand the name of the chart.
*/}}
{{- define "external-dns.fullname" -}}
{{- printf "external-dns-%s" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "external-dns.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Service account name
*/}}
{{- define "external-dns.serviceAccountName" -}}
{{- default "external-dns" (get .Values.serviceAccount "name") | quote -}}
{{- end -}} 