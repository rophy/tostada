{{/*
Standard chart helpers.
*/}}
{{- define "tostada.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
