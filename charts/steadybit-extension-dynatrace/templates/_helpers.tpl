{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "dynatrace.secret.name" -}}
{{- default "steadybit-extension-dynatrace" .Values.dynatrace.existingSecret -}}
{{- end -}}
