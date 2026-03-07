{{/*
Sidecar container definition — embed this in your existing Deployment spec.

Usage in a Deployment (add to spec.template.spec.containers[]):
  {{- include "stracectl.sidecar" . | nindent 8 }}

And add to spec.template.spec:
  shareProcessNamespace: true
*/}}
{{- define "stracectl.sidecar" -}}
- name: stracectl
  image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  args:
    - attach
    - --serve
    - ":{{ .Values.port }}"
    - {{ .Values.targetPID | quote }}
  ports:
    - name: http
      containerPort: {{ .Values.port }}
      protocol: TCP
  securityContext:
    capabilities:
      add:
        - SYS_PTRACE
    runAsNonRoot: false
  resources:
    {{- toYaml .Values.resources | nindent 4 }}
  livenessProbe:
    httpGet:
      path: /healthz
      port: {{ .Values.port }}
    initialDelaySeconds: 5
    periodSeconds: 10
  readinessProbe:
    httpGet:
      path: /healthz
      port: {{ .Values.port }}
    initialDelaySeconds: 2
    periodSeconds: 5
{{- end }}
