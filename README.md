# 🖥 Mac Toolkit — Go CLI

[![CI](https://github.com/arheanja-ops/mac-toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/arheanja-ops/mac-toolkit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/arheanja-ops/mac-toolkit.svg)](https://pkg.go.dev/github.com/arheanja-ops/mac-toolkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

CLI nativo para macOS: limpieza de disco, monitoreo del sistema, y optimización.
Incluye una **TUI interactiva estilo k9s** (dos paneles + header en vivo). Binario
único, cero dependencias de runtime.

## Instalación

### Con `go install` (recomendado)

```bash
go install github.com/arheanja-ops/mac-toolkit@latest
mac-toolkit --help   # queda en $(go env GOPATH)/bin
```

### Desde el código fuente

```bash
git clone https://github.com/arheanja-ops/mac-toolkit.git
cd mac-toolkit
go build -o bin/toolkit .
./bin/toolkit --help

# O instalar globalmente:
make install  # → /usr/local/bin/toolkit
```

> Requisitos: macOS (Apple Silicon o Intel) y Go 1.25+.

### Versión (`--version`)

```bash
mac-toolkit --version
```

La versión se resuelve así (`cmd/root.go`, `resolveBuildInfo`):

- **Binario del release (GoReleaser):** versión, commit y fecha inyectados por
  `-ldflags`.
- **`go install ...@vX.Y.Z`:** no hay ldflags, así que la versión se lee de
  `runtime/debug.ReadBuildInfo()`. Verificado con `@v1.7.1`:
  `mac-toolkit version v1.7.1 (commit none, built unknown)`.
- **Build local en un checkout (`go build`):** el toolchain puede derivar la
  versión del VCS (p. ej. `v1.7.1+dirty`) y completar commit y fecha.

Commit y fecha de compilación reales (inyectados) solo vienen en los binarios del
release; con `go install` aparecen como `none`/`unknown`.

## Uso

### TUI interactiva (estilo k9s) — recomendado

```bash
mac-toolkit          # sin argumentos abre la TUI de dos paneles
mac-toolkit tui      # equivalente explícito
```

Al abrir entra a una pantalla de bienvenida con un resumen del sistema. El layout:

- **Header en vivo** (se refresca solo): uso de disco, memoria, CPU y estado térmico.
- **Panel izquierdo** — acciones agrupadas en Disk / Clean / Monitors / Reports.
- **Panel derecho** — resultado de la acción seleccionada, con scroll.

Acciones del panel izquierdo (`internal/tui/actions.go`): **Disk** (Analyze,
Status, Doctor) · **Clean** (Preview, Select & clean) · **Monitors** (Battery,
System, Processes, Network) · **Reports** (Last report) · **Session** (Quit / Salir).

**Teclas en modo normal** (`internal/tui/model.go`):

| Tecla | Acción |
|-------|--------|
| `↑` `↓` | Mover (en el panel con foco) |
| `Tab` | Cambiar de panel |
| `⏎` | Ejecutar la acción seleccionada |
| `/` | Filtrar acciones (`Esc`/`⏎` cierran el filtro) |
| `a` | Analyze |
| `p` | Preview de limpieza (dry-run) |
| `c` | Abrir el modo selección de limpieza |
| `d` | Doctor (diagnóstico priorizado, solo lectura) |
| `r` | Refrescar Doctor (solo si el panel Doctor está abierto) |
| `q` · `Ctrl+C` · `Esc` | Salir (con filtro activo, `Esc` solo cierra el filtro) |

**Teclas en el modo selección de limpieza:** `↑↓` (o `k`/`j`) mover · `espacio`
marcar/desmarcar · `a` marcar todos los seguros · `n` ninguno · `⏎` continuar ·
`Esc` cancelar.

**Modal de confirmación:** escribir `si` + `⏎` borra; `Esc` o cualquier otra
entrada cancela sin borrar.

El panel **Doctor** muestra tres grupos (PERFORMANCE, QUICK WINS, REQUIERE
REVISIÓN), el contexto de disco y el motor usado (Groq o heurístico). Cada `d`/`r`
re-analiza sin caché. Es solo lectura y se adapta hasta 80x24.

**Limpieza interactiva (TUI-2).** Además de analizar, la TUI puede **borrar**:

- `Preview` muestra el plan de limpieza (dry-run, nunca borra).
- `Clean (select & delete)` abre un modo selección con checkboxes. **Solo los
  items `safe` son marcables**; `warn`/`danger` se muestran atenuados con su
  razón y no se pueden marcar. Teclas: `espacio` marca · `a` todos los seguros ·
  `n` ninguno · `⏎` continuar · `Esc` cancelar.
- Al continuar aparece un **modal de confirmación**: debes escribir `si` y pulsar
  `⏎` para borrar (`core.IsAffirmative` también acepta `sí`/`yes`/`y`, igual que el
  CLI). Cualquier otra cosa cancela. El borrado pasa por la misma blacklist y
  escribe el mismo audit log que el CLI.

> Garantía: la TUI solo borra items `safe`, tras selección explícita y
> confirmación `si`. Detalles en [`docs/SAFETY.md`](docs/SAFETY.md).

```bash
mac-toolkit menu     # menú lineal (promptui) — fallback si no hay TUI
```

### Diagnóstico con IA — `doctor`

```bash
mac-toolkit doctor            # plan priorizado (performance / quick wins / revisión)
mac-toolkit doctor --no-ai    # fuerza el motor heurístico (sin IA)
mac-toolkit doctor --json     # salida estructurada para scripting
```

`doctor` es **solo lectura**: orquesta análisis + monitores y produce un plan
priorizado, nunca borra ni ejecuta acciones. Usa Groq si hay `GROQ_API_KEY`; si
no, o si la llamada falla o excede 20 s, cae en silencio a un motor heurístico
determinista (sin depender de internet). También está disponible en la TUI
(`d`) y como tool MCP `mac_doctor`. Configuración por entorno:

| Variable | Default | Propósito |
|----------|---------|-----------|
| `GROQ_API_KEY` | — (vacío → heurístico) | Autenticación Groq |
| `GROQ_MODEL` | `openai/gpt-oss-20b` | Modelo de inferencia |
| `GROQ_BASE_URL` | `https://api.groq.com/openai/v1` | Endpoint OpenAI-compatible |

> Privacidad: al usar IA solo se envían **agregados** (tamaños por dominio,
> métricas de sistema) — nunca rutas, nombres de archivo ni PIDs.

### Comandos directos

```bash
# Disk cleanup
mac-toolkit analyze                          # Análisis completo
mac-toolkit analyze --domain dev_caches      # Solo un dominio
mac-toolkit analyze --save                   # Guardar reporte MD + JSON
mac-toolkit clean --execute --mode checklist # Limpieza con checklist (dry-run por defecto)
mac-toolkit full --execute                   # Analizar + guardar + limpiar

# Diagnóstico
mac-toolkit doctor       # Plan de salud priorizado (solo lectura)

# Docker
mac-toolkit docker backup <contenedor>            # Dump de BD (postgres/mysql) a ~/docker-backups/
mac-toolkit docker cleanup --keep a,b             # Dry-run; --execute para borrar
mac-toolkit docker compact                        # Docker.raw: tamaño virtual vs uso real

# Monitors
mac-toolkit battery      # Salud, ciclos, temperatura
mac-toolkit system       # CPU, memoria, swap, estado térmico (Normal/Throttled/Warning/Unknown)
mac-toolkit processes    # Top 10 por CPU y memoria
mac-toolkit network      # WiFi, estadísticas, conectividad

# Info
mac-toolkit status       # Dominios registrados y niveles de riesgo
mac-toolkit report --last # Último reporte guardado
```

## Dominios de análisis (11)

| Dominio | Qué analiza | Riesgo |
|---------|-------------|--------|
| `disk` | Uso APFS del volumen | — |
| `ollama` | Modelos LLM descargados | 🔴 danger |
| `docker` | Imagen virtual Docker.raw | 🔴 danger |
| `dev_caches` | npm, pip, brew caches | 🟢 safe |
| `browser` | Chrome, Safari, Firefox, Edge | 🟢 safe |
| `logs` | Logs del sistema >7 días | 🟢 safe |
| `repos` | node_modules, .venv, __pycache__ | 🟢 safe |
| `xcode` | DerivedData, Simulators, Archives | 🟢/🟡 |
| `downloads` | Archivos grandes, ZIPs, duplicados | 🟡 warn |
| `appsupport` | Application Support >50MB | 🟡 warn |
| `trash` | Papelera ~/.Trash | 🟡 warn |

### Estados térmicos

`system`, el header de la TUI, `doctor` y `mac_system` clasifican `pmset -g therm`
(`internal/monitor/thermal.go`) en cuatro estados:

| Estado | Significado |
|--------|-------------|
| `Normal` | Sin avisos térmicos (o `CPU_Speed_Limit = 100`) |
| `Throttled` | `CPU_Speed_Limit` < 100: el CPU está limitado |
| `Warning` | Hay un nivel de aviso realmente registrado |
| `Unknown` | Salida vacía o irreconocible, o `pmset` falló |

Solo `Throttled` y `Warning` se tratan como alerta; `Normal` y `Unknown` nunca se
pintan ni cuentan como tal. Es lectura pura: no modifica nada del sistema.

## Seguridad

- **Dry-run por defecto** — sin `--execute` nunca borra nada
- **Confirmación final** — con `--execute`, además de la aprobación por modo, se
  exige escribir `si` tras ver el número de elementos y el tamaño total
- **Blacklist** permanente: `/System`, `/usr`, `/bin`, `/sbin`, `/private/var/db`
  (más los bundles `com.apple.dock`, `com.apple.finder`, `com.apple.spotlight`)
- **Preview** antes de borrar con tamaño, riesgo y antigüedad
- **TUI** — analiza y monitorea; solo borra items `safe`, tras selección explícita
  y el modal `si`, con blacklist y audit log
- **`doctor`** — estrictamente solo lectura (CLI, TUI y MCP)
- **MCP** — las tools destructivas usan `dry_run=true` por defecto y exigen opt-in (`MAC_TOOLKIT_MCP_ALLOW_DELETE=1`) + `confirm_plan`
- **Docker** — `docker cleanup --execute` muestra el plan, exige confirmación `si` y escribe audit log
- **Audit log** JSON por sesión con UUID (`clean` y TUI)

> Garantías completas y verificables (qué es solo lectura, qué requiere
> confirmación, y dónde está en el código): **[docs/SAFETY.md](docs/SAFETY.md)**.

## Modos de aprobación (`--mode`)

- `deal` — resumen total → aprueba por categoría (default)
- `category` — s/N por cada dominio
- `item` — s/N por cada archivo
- `checklist` — selección por números o "all"/"todos"

## Arquitectura

```
mac-toolkit/
├── cmd/              # Cobra CLI (subcomandos, TUI, menú lineal, doctor, MCP)
├── internal/
│   ├── core/         # config, models, runner (goroutines), logger, approval, IsAffirmative
│   ├── analyzer/     # Interfaz + 11 domain analyzers (auto-registrados via init())
│   ├── monitor/      # Interfaz + 4 monitors (battery, system, processes, network) + parser térmico
│   ├── tui/          # TUI estilo k9s (Bubble Tea): 2 paneles, header en vivo, clean, doctor
│   ├── ai/           # doctor: snapshot sin PII, motor heurístico, cliente Groq, collect
│   ├── docker/       # backup, cleanup y compact de Docker
│   ├── cleaner/      # Blacklist + GenericCleaner
│   └── reporter/     # terminal, markdown, json, audit
├── docs/             # requirements, design, tasks, SAFETY, RELEASE_WORKFLOW, specs/, guías MCP
├── .kiro/            # Skills, agente y steering para Kiro CLI
│   ├── agents/       # mac-toolkit-dev (agente Go dev)
│   ├── skills/       # 6 skills: analyze-domain, add-analyzer, add-monitor,
│   │                 #   toolkit-test, toolkit-build, mac-optimize
│   └── steering/     # Convenciones del proyecto
├── main.go
├── go.mod
└── Makefile
```

## MCP Server — Chat Integration

El toolkit se puede usar como MCP server, exponiendo todas sus capacidades como herramientas invocables desde Kiro, Claude Desktop, o cualquier cliente MCP.

```bash
# Ejecutar como MCP server (stdio)
mac-toolkit mcp
```

### Herramientas MCP disponibles (12)

Fuente: `mcp.AddTool` en `cmd/mcp.go`.

Solo lectura (9) — nunca borran ni escriben:

| Tool | Descripción | Parámetros |
|------|-------------|------------|
| `mac_analyze` | Análisis de disco por dominio o completo | `domain` (opcional) |
| `mac_battery` | Salud de batería, ciclos, temperatura, voltaje | — |
| `mac_system` | CPU, memoria, swap, estado térmico, modelo y chip | — |
| `mac_processes` | Top 10 por CPU y memoria + presión de memoria | — |
| `mac_network` | WiFi, tráfico, conexiones, online | — |
| `mac_status` | Dominios registrados y niveles de riesgo | — |
| `mac_clean_preview` | Preview de limpieza (dry-run seguro) | `domain` (opcional) |
| `mac_docker_compact` | Docker.raw: tamaño virtual vs uso real + instrucciones | — |
| `mac_doctor` | Diagnóstico priorizado (performance / quick wins / revisión); Groq si hay `GROQ_API_KEY`, si no heurístico | `no_ai` (opcional) |

Destructivas (2) — `dry_run=true` por defecto (preview + `plan_digest`). `dry_run=false` solo ejecuta con `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` en el servidor **y** `confirm_plan` = `plan_digest` vigente:

| Tool | Descripción | Parámetros |
|------|-------------|------------|
| `mac_clean_batch` | Borra solo items `safe_to_delete` en varios dominios | `domains` (opcional), `dry_run` (default `true`), `confirm_plan` (requerido con `dry_run=false`) |
| `mac_docker_cleanup` | Elimina recursos Docker excepto los de `keep` | `keep` (requerido), `dry_run` (default `true`), `confirm_plan` (requerido con `dry_run=false`) |

Escritura sin borrado (1) — **sin** parámetro `dry_run`:

| Tool | Descripción | Parámetros |
|------|-------------|------------|
| `mac_docker_backup` | Dump de BD de un contenedor en ejecución (pg_dumpall/mysqldump) a `~/docker-backups/`; no borra nada | `container` (requerido) |

> Recomendación: auto-aprueba en tu cliente solo las 9 de solo lectura. Las 2
> destructivas requieren confirmación explícita del usuario antes de invocarse con
> `dry_run=false`, y `mac_docker_backup` escribe un archivo, así que conviene
> confirmarla también.

**Habilitar borrados por MCP (opt-in).** Sin esto, `dry_run=false` se rechaza. Añade la
variable al `env` del servidor en tu configuración MCP (solo tú puedes; el agente no):

```json
"mac-toolkit": {
  "command": "mac-toolkit",
  "args": ["mcp"],
  "env": { "MAC_TOOLKIT_MCP_ALLOW_DELETE": "1" }
}
```

Flujo: preview → revisas el plan y su `plan_digest` → `dry_run=false` con
`confirm_plan=<plan_digest>`. Si el estado cambió, se rechaza. Cada ejecución real
escribe un audit log (`mcp`). Alternativa en terminal:
`mac-toolkit docker cleanup --keep X --execute` / `mac-toolkit clean --execute`.

### Configurar en Kiro

Desde el directorio `mac-toolkit/`, usa el agente `mac-ops`:

```bash
kiro-cli chat --agent mac-ops
```

O agrega manualmente el MCP server a cualquier agente:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

> Si instalaste con `go install`, `mac-toolkit` estará en `$(go env GOPATH)/bin`
> (asegúrate de tenerlo en el `PATH`). Alternativamente usa la ruta absoluta al
> binario, p. ej. `$(go env GOPATH)/bin/mac-toolkit` o, si compilaste con
> `make build`/`make install`, `./bin/toolkit` o `/usr/local/bin/toolkit`.

### Configurar en otros clientes (Claude, VS Code, Cursor, Windsurf, Zed)

El servidor es un binario stdio genérico — cualquier cliente MCP lo lanza con
`command: mac-toolkit`, `args: ["mcp"]`. Ejemplo (Claude Desktop / Cursor, clave `mcpServers`):

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

VS Code usa la clave `servers` (no `mcpServers`); Claude Code se configura con
`claude mcp add mac-toolkit -- mac-toolkit mcp`.

Guía completa por cliente: **[docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)**.

### Ejemplos desde chat

```
> ¿cómo está mi mac?          → mac_system + mac_battery + mac_processes
> analiza el disco             → mac_analyze (todos los dominios)
> qué puedo limpiar            → mac_clean_preview
> revisa los caches de browser → mac_analyze domain=browser
> revisa la red                → mac_network
> diagnostica mi mac           → mac_doctor
```

## Dependencias

- **cobra v1.8.1** — CLI framework
- **go-sdk v1.7.0** — MCP server (Model Context Protocol oficial)
- **bubbletea · lipgloss · bubbles** (charmbracelet) — TUI estilo k9s (`internal/tui/`)
- **promptui** — menú lineal (`mac-toolkit menu`)
- Go stdlib para todo lo demás (exec.Command, filepath.WalkDir, goroutines, net/http)

## Desarrollo

```bash
make all        # lint + test + build
make test       # go test ./...
make build      # go build -o bin/toolkit
make lint       # go vet ./...
```

> **Flujo de contribución y release** (PR → merge → tag → build): ver
> **[docs/RELEASE_WORKFLOW.md](docs/RELEASE_WORKFLOW.md)**. Cada cambio va por PR;
> cada versión se publica con un tag `vX.Y.Z` que dispara GoReleaser.

### Kiro Skills disponibles

| Skill | Descripción |
|-------|-------------|
| `/analyze-domain <d>` | Ejecutar análisis de un dominio |
| `/add-analyzer <name>` | Agregar nuevo analyzer |
| `/add-monitor <name>` | Agregar nuevo monitor |
| `/toolkit-test [pkg]` | Ejecutar tests y analizar fallos |
| `/toolkit-build [release]` | Build completo con verificación |
| `/mac-optimize [clean]` | Análisis completo de optimización |

### Agregar un nuevo analyzer

1. Crear `internal/analyzer/myanalyzer.go` implementando la interfaz `Analyzer`
2. Llamar `Register(&MyAnalyzer{})` en `init()`
3. Se auto-registra — no necesita cambios en ningún otro archivo
4. O usa: `/add-analyzer myanalyzer`

## Tests

```bash
go test ./... -v -count=1
# 108 tests de nivel superior (167 contando subtests) en 7 paquetes con tests:
# cmd, ai, analyzer, cleaner, core, monitor, tui
```

---

*Mac Toolkit v1.7 — Go CLI + TUI (k9s-style) + `doctor` (IA/Groq) + MCP para macOS • [arheanja-ops](https://github.com/arheanja-ops)*

Ver [TOOLKIT_GUIDE.md](TOOLKIT_GUIDE.md) para documentación completa: configuración MCP en Kiro Desktop/CLI, herramientas disponibles, seguridad, skills, y guía de extensibilidad.
