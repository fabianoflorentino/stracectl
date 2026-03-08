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
    privileged: false
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: {{ .Values.securityContext.readOnlyRootFilesystem }}
    runAsNonRoot: false
    runAsUser: 0
    seccompProfile:
      type: {{ .Values.securityContext.seccompProfile }}
    capabilities:
      drop:
        - ALL
      add:
        - SYS_PTRACE
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
