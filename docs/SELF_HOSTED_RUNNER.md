# Self-hosted runner para eBPF

Este documento descreve como provisionar e configurar um runner self-hosted para executar builds e testes que carregam ou interagem com eBPF no kernel.

Por que usar self-hosted
- GitHub-hosted runners são ótimos para build/CI, mas não permitem operações privilegiadas no kernel (capabilities, containers privilegiados, etc.).
- Testes de integração que carregam BPF exigem capacidades como `CAP_BPF`, `CAP_PERFMON` ou execução como `root`.

Requisitos mínimos do host
- Kernel Linux moderno (recomendo >= 5.8). Verifique `uname -r`.
- Pacotes: `clang`, `llvm`, `libbpf-dev`, `linux-headers-$(uname -r)`, `make`, `git`, `go`.
- Espaço em disco suficiente para toolchain / artefatos.

Instalação rápida do runner
1. Crie um diretório para o runner e baixe a distribuição oficial:

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
RUNNER_VER=2.305.0
curl -sL -o actions-runner.tar.gz \
  https://github.com/actions/runner/releases/download/v${RUNNER_VER}/actions-runner-linux-x64-${RUNNER_VER}.tar.gz
tar xzf actions-runner.tar.gz
```

2. Registre o runner no repositório (gere um token temporário no GitHub UI: Settings → Actions → Runners → New self-hosted runner):

```bash
./config.sh --url https://github.com/ORG/REPO --token YOUR_TOKEN --labels self-hosted,linux,ebpf --name my-runner
```

3. Instale e inicie como serviço (execute como `root` para instalar o unit systemd):

```bash
sudo ./svc.sh install
sudo ./svc.sh start
```

Systemd: exportar variáveis de proxy (exemplo)
Se o runner estiver atrás de proxy HTTP/HTTPS crie um drop-in unit para injetar as variáveis de ambiente:

```bash
SERVICE=actions.runner.ORG-REPO.my-runner.service
sudo mkdir -p /etc/systemd/system/${SERVICE}.d
sudo tee /etc/systemd/system/${SERVICE}.d/proxy.conf > /dev/null <<'EOF'
[Service]
Environment="HTTP_PROXY=http://proxy.example:3128"
Environment="HTTPS_PROXY=http://proxy.example:3128"
Environment="NO_PROXY=localhost,127.0.0.1,github.com,api.github.com"
Environment="http_proxy=http://proxy.example:3128"
Environment="https_proxy=http://proxy.example:3128"
Environment="no_proxy=localhost,127.0.0.1,github.com,api.github.com"
EOF

sudo systemctl daemon-reload
sudo systemctl restart "${SERVICE}"
sudo journalctl -u "${SERVICE}" -f
```

Labels: workflow eBPF
No workflow usamos o label `ebpf`. As jobs que precisam de um runner capaz de carregar/rodar eBPF devem ter:

```yaml
runs-on: [self-hosted, linux, ebpf]
```

Segurança e privilégios
- Para carregar programas eBPF o processo precisa de privilégios de kernel. A forma mais simples é executar o runner como `root`. Alternativas:
  - executar jobs em VMs separadas (recomendado) ou containers privilegiados com `--privileged` e capabilities adequadas.
  - conceder capabilities específicas (`CAP_BPF`, `CAP_PERFMON`) à sessão que executa os testes (mais complexo).

Instalar `bpf2go` e ferramenta local
```bash
export PATH=$PATH:$(go env GOPATH)/bin
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

Diagnóstico (se o job estiver "Waiting for a runner")
- Verifique runners registrados no repositório (local ou remote):
```bash
gh api repos/ORG/REPO/actions/runners --jq '.runners[] | {name: .name, status: .status, busy: .busy, labels: .labels}'
```
- No host do runner, verifique o serviço systemd:
```bash
systemctl list-units --type=service | grep actions.runner
sudo systemctl status actions.runner.* --no-pager
sudo journalctl -u actions.runner.* --since "10 minutes ago"
```

Se o runner não tiver a label `ebpf`, reconfigure com as labels corretas:
```bash
# no diretório do runner
./config.sh remove --unattended
./config.sh --url https://github.com/ORG/REPO --token YOUR_TOKEN --labels self-hosted,linux,ebpf --name my-runner
sudo ./svc.sh start
```

Coletar logs se houver erros de download (401)
1. No host do runner, salve os logs do serviço para análise:
```bash
sudo journalctl -u actions.runner.* --since "10 minutes ago" > /tmp/runner_journal.txt
```
2. Reproduzir a requisição falhada (substitua URL vista nos logs) para capturar cabeçalhos:
```bash
curl -v -D /tmp/curl_headers.txt -o /tmp/curl_body.bin 'https://api.github.com/repos/actions/setup-go/tarball/<SHA>' -H 'Accept: application/vnd.github+json'
```
3. Anexe `/tmp/runner_journal.txt` e `/tmp/curl_headers.txt` ao issue/PR para análise.

Notas finais
- Recomendo provisionar o runner em uma VM dedicada (cloud) para simplificar permissões e isolamento.
- Use labels (`ebpf`) para direcionar jobs sensíveis.

Se quiser, eu:
- adiciono este arquivo ao repo e faço commit + push, e
- re-executo o workflow `eBPF Build & Generate` e monitoro o `integration` job até ele ser atendido ou falhar (coleto logs se necessário).
