{{/*
Build image reference from registry/repository/tag with global.imageRegistry override.
Usage: {{ include "tostada.image" (dict "image" .Values.path.to.image "global" .Values.global) }}
*/}}
{{- define "tostada.image" -}}
{{- $registry := .image.registry -}}
{{- if and .global .global.imageRegistry -}}
  {{- $registry = .global.imageRegistry -}}
{{- end -}}
{{- if $registry -}}
  {{- printf "%s/%s:%s" $registry .image.repository (.image.tag | toString) -}}
{{- else -}}
  {{- printf "%s:%s" .image.repository (.image.tag | toString) -}}
{{- end -}}
{{- end -}}
