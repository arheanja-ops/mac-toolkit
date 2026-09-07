# Proyecto Mac Toolkit + MCP Servers — Estado al 2026-08-29

> **Nota (actualización 2026-09-07):** esta es una bitácora histórica. El estado
> actual difiere en: el comando instalado es **`mac-toolkit`** (no `toolkit`); el
> servidor MCP expone **11 tools** (no 7); el timeout por analyzer es **180s**; el
> repositorio vive en `github.com/arheanja-ops/mac-toolkit`. Ver README.md y
> CHANGELOG.md para el estado vigente.

## Quién
- Usuario: Jaime Henao (jaime.henao)
- Equipo: BA (British Airways) — Platform/DevOps
- Compañero: Sebastián Álvarez Hernández (sebastian.ah) — creó el Backstage MCP bridge

## Qué se construyó hoy

### 1. Mac Toolkit — CLI Go completa
- **Ubicación**: `/Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/`
- **Binario**: `bin/toolkit` (15MB, Go, single binary)
- **Migración de**: Python `mac_toolkit_pro/` (no se tocó el proyecto original)
- **CLI**: Cobra con 10 subcomandos: analyze, clean, full, status, report, battery, system, processes, network, mcp
- **11 analyzers**: disk, ollama, docker, browser, logs, downloads, appsupport, repos, dev_caches, xcode, trash
- **4 monitors**: battery (ioreg/pmset), system (vm_stat/top/system_profiler), processes (ps), network (netstat/airport)
- **MCP server nativo**: `toolkit mcp` — 7 tools via SDK oficial go-sdk v1.7.0
- **Tests**: 19 passing (core, analyzer, cleaner)
- **Deps**: cobra v1.8.1 + go-sdk v1.7.0
- **Kiro integration**: 2 agentes, 6 skills, 1 steering file

### 2. MCP Tools expuestas (mac-toolkit)
- `mac_analyze` — análisis disco (domain opcional)
- `mac_battery` — batería
- `mac_system` — CPU/RAM/swap/thermal
- `mac_processes` — top procesos
- `mac_network` — WiFi/tráfico/conectividad
- `mac_status` — dominios y riesgos
- `mac_clean_preview` — preview limpieza (dry-run)

### 3. Kiro Crew — MCP servers configurados (16 total)

**Día 1 (2026-08-28):**
- ✅ `mac-toolkit` — Online, 7 tools, binario Go directo stdio
- ✅ `aws-docs` — Online, 5 tools, `uvx awslabs.aws-documentation-mcp-server@latest`
- ⏳ `kubernetes` — configurado, necesita VPN BA (cluster EKS eu-west-1)

**Día 2 (2026-08-29) — agregados:**
- ✅ `aws-cloudwatch` — `uvx awslabs.cloudwatch-mcp-server@latest`
- ✅ `aws-ecs` — `uvx awslabs.ecs-mcp-server@latest`
- ✅ `aws-eks` — `uvx awslabs.eks-mcp-server@latest`
- ✅ `aws-sns-sqs` — `uvx awslabs.amazon-sns-sqs-mcp-server@latest`
- ✅ `aws-iam` — `uvx awslabs.iam-mcp-server@latest`
- ✅ `aws-billing` — `uvx awslabs.billing-cost-management-mcp-server@latest`
- ✅ `aws-cloudtrail` — `uvx awslabs.cloudtrail-mcp-server@latest`
- ✅ `github` — Docker `ghcr.io/github/github-mcp-server`, PAT via env var `GITHUB_PERSONAL_ACCESS_TOKEN`
- ⏳ `backstage` — bridge de Sebastián, necesita Backstage en localhost:7007 + MCP_TOKEN
- 🔧 `MCP_DOCKER` — corregido (command antes de args), Docker MCP Toolkit gateway

**Internos (siempre online):**
- ✅ `kirocrew-cron` — scheduler
- ✅ `kirocrew-core` — core agent
- 🚫 `fetch` — deshabilitado

- Config en: `~/.kiro/agents/kirocrew.json`

### 4. Kiro Desktop — MCP config
- Global: `~/.kiro/settings/mcp.json` tiene `mac-toolkit` habilitado
- Workspace: `mac-toolkit/.kiro/settings/mcp.json` también tiene config

## Archivos clave creados
- `mac-toolkit/cmd/mcp.go` — MCP server (7 tools, SDK oficial)
- `mac-toolkit/.kiro/agents/mac-ops.json` — agente operaciones (Ctrl+Shift+M)
- `mac-toolkit/.kiro/agents/mac-toolkit-dev.json` — agente desarrollo (Ctrl+Shift+T)
- `mac-toolkit/docs/MCP_SETUP.md` — guía conexión Kiro Crew/Desktop/CLI
- `mac-toolkit/docs/MCP_SERVERS_SETUP.md` — guía 3 MCP servers (K8s, AWS, Backstage)
- `mac-toolkit/TOOLKIT_GUIDE.md` — guía completa 499 líneas
- `mac-toolkit/README.md` — readme del proyecto

## Pendiente para el martes
1. **Kubernetes MCP**: conectar VPN BA → verificar que `kubernetes` MCP server conecte
   - JSON ya configurado en kirocrew.json
   - node directo (no npx)
   - Cluster: EKS eu-west-1, contexto: dev1

2. **Backstage MCP**: coordinar con Sebastián
   - Necesita: Backstage corriendo en localhost:7007
   - Generar MCP_TOKEN: `export MCP_TOKEN=$(node -p 'require("crypto").randomBytes(24).toString("base64")')`
   - Verificar path del bridge (actualmente apunta a máquina de Sebastián)
   - Agregar MCP_TOKEN al `~/.zshrc`

3. **Verificar GitHub MCP**: reiniciar Kiro Crew y confirmar que conecta con el PAT

4. **Verificar AWS MCPs**: confirmar que los 7 nuevos servers se conectan online

## Contexto técnico
- macOS, Apple M1 Pro, 16GB RAM
- Go instalado
- uvx: `/Users/jaime.henao/.local/bin/uvx`
- node: `/Users/jaime.henao/.nvm/versions/node/v22.18.0/bin/node`
- npm global: `/Users/jaime.henao/.nvm/versions/node/v22.18.0/lib/node_modules/`
- Kiro Crew: `/Applications/KiroCrew.app`
- AWS: profile configurado, kubeconfig → EKS eu-west-1
- Python original intacto en `mac_toolkit_pro/`

## Patrón para agregar MCP en Kiro Crew
1. Kiro Crew → Agent Capabilities → Connections → MCP Servers → Add Custom
2. Pegar JSON → Apply
3. SIEMPRE usar rutas absolutas para commands (Kiro Crew no hereda PATH)
4. Si npx da timeout, instalar globalmente con npm y usar node directo
5. Para uvx: usar `/Users/jaime.henao/.local/bin/uvx`

---

## Actualización 2026-09-01 — Unificación de MCP servers AWS

### Qué se hizo
Se unificaron los 7 MCP servers AWS especializados en un solo server `aws-api`
(`awslabs.aws-api-mcp-server`). Se mantuvo `aws-docs` (documentación) porque el
server unificado no la incluye.

### Cambio en `~/.kiro/agents/kirocrew.json`
- **Eliminados** (7): `aws-cloudwatch`, `aws-ecs`, `aws-eks`, `aws-sns-sqs`,
  `aws-iam`, `aws-billing`, `aws-cloudtrail`
- **Agregado** (1): `aws-api`
- **Mantenido**: `aws-docs`
- Referencias `@aws-*` en `tools`/`allowedTools` actualizadas (sin huérfanas)
- Backup: `~/.kiro/agents/kirocrew.json.bak-20260901-105014`

### Config del server unificado
```json
"aws-api": {
  "command": "/Users/jaime.henao/.local/bin/uvx",
  "args": ["awslabs.aws-api-mcp-server@latest"],
  "env": {
    "AWS_REGION": "eu-west-1",
    "AWS_API_MCP_PROFILE_NAME": "ReadOnly-851999171650",
    "READ_OPERATIONS_ONLY": "true",
    "FASTMCP_LOG_LEVEL": "ERROR"
  }
}
```

### Tools que expone `aws-api`
- `call_aws` — ejecuta cualquier comando AWS CLI (validado, read-only por ahora)
- `suggest_aws_commands` — sugiere comandos CLI desde lenguaje natural
- Cubre **todos** los servicios AWS vía CLI (no solo los 7 anteriores)

### Seguridad
- `READ_OPERATIONS_ONLY=true` → solo operaciones de lectura. Para habilitar
  escritura en el futuro: quitar esa var (o ponerla en `"false"`) y considerar
  `REQUIRE_MUTATION_CONSENT=true`.
- Perfil `ReadOnly-851999171650` (cuenta principal, solo lectura).

### Autenticación — Common Fate / Granted + AWS SSO
- Perfiles son SSO-based (`sso-session nexus`, start URL `https://ba-pr.awsapps.com/start`).
- `granted`/`assume` en `/opt/homebrew/bin/`.
- El MCP hereda credenciales vía la cadena boto3 → lee token SSO de `~/.aws/sso/cache/`.
- **Requiere sesión SSO activa**. Renovar con:
  - `aws sso login --sso-session nexus`, o
  - `assume ReadOnly-851999171650`
- Kiro Crew es app macOS (no hereda el shell) → depende del token SSO cacheado.

### Pendiente de verificación
- Al momento del cambio la sesión SSO estaba EXPIRADA
  (`Token has expired and refresh failed`).
- Tras hacer login SSO: reiniciar Kiro Crew y confirmar que `aws-api` aparece
  Online con sus 2 tools. Probar prompt: "lista mis instancias EC2 en eu-west-1".
