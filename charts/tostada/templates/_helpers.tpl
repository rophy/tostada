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

{{/*
Merge global.podAnnotations with extra annotations (e.g. checksums).
Usage: include "tostada.podAnnotations" (dict "global" .Values.global "extra" $extraDict)
*/}}
{{- define "tostada.podAnnotations" -}}
{{- $merged := dict -}}
{{- if and .global .global.podAnnotations -}}
  {{- $merged = merge $merged .global.podAnnotations -}}
{{- end -}}
{{- if .extra -}}
  {{- $merged = merge $merged .extra -}}
{{- end -}}
{{- if $merged -}}
{{- toYaml $merged -}}
{{- end -}}
{{- end -}}
