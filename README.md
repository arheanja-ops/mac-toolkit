# 🖥 Mac Toolkit — Go CLI

[![CI](https://github.com/arheanja-ops/mac-toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/arheanja-ops/mac-toolkit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/arheanja-ops/mac-toolkit.svg)](https://pkg.go.dev/github.com/arheanja-ops/mac-toolkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

CLI nativo para macOS: limpieza de disco, monitoreo del sistema, y optimización. Binario único, cero dependencias de runtime.

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

## Uso

```bash
# Menú interactivo con flechas (↑↓), agrupado en Disk / Monitors / Reports
mac-toolkit

# Disk cleanup
mac-toolkit analyze                          # Análisis completo
mac-toolkit analyze --domain dev_caches      # Solo un dominio
mac-toolkit analyze --save                   # Guardar reporte MD + JSON
mac-toolkit clean --execute --mode checklist # Limpieza con checklist
mac-toolkit full --execute                   # Analizar + guardar + limpiar

# Monitors
mac-toolkit battery      # Salud, ciclos, temperatura
mac-toolkit system       # CPU, memoria, swap, estado térmico
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

## Seguridad

- **Dry-run por defecto** — sin `--execute` nunca borra nada
- **Blacklist** permanente: `/System`, `/usr`, `/bin`, `/sbin`, `/private/var/db`
- **Preview** antes de borrar con tamaño, riesgo y antigüedad
- **Audit log** JSON por sesión con UUID

## Modos de aprobación (`--mode`)

- `deal` — resumen total → aprueba por categoría (default)
- `category` — s/N por cada dominio
- `item` — s/N por cada archivo
- `checklist` — selección por números o "all"/"todos"

## Arquitectura

```
mac-toolkit/
├── cmd/              # Cobra CLI (9 subcomandos + menú interactivo)
├── internal/
│   ├── core/         # config, models, runner (goroutines), logger, approval
│   ├── analyzer/     # Interfaz + 11 domain analyzers (auto-registrados via init())
│   ├── monitor/      # Interfaz + 4 monitors (battery, system, processes, network)
│   ├── cleaner/      # Blacklist + GenericCleaner
│   └── reporter/     # terminal, markdown, json, audit
├── docs/             # requirements.md, design.md, tasks.md
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

### Herramientas MCP disponibles (11)

Solo lectura / seguras (dry-run, nunca borran):

| Tool | Descripción | Parámetros |
|------|-------------|------------|
| `mac_analyze` | Análisis de disco por dominio o completo | `domain` (opcional) |
| `mac_battery` | Salud de batería, ciclos, temperatura, voltaje | — |
| `mac_system` | CPU, memoria, swap, thermal, modelo y chip | — |
| `mac_processes` | Top 10 por CPU y memoria + presión de memoria | — |
| `mac_network` | WiFi, tráfico, conexiones, online | — |
| `mac_status` | Dominios registrados y niveles de riesgo | — |
| `mac_clean_preview` | Preview de limpieza (dry-run seguro) | `domain` (opcional) |
| `mac_docker_compact` | Docker.raw: tamaño virtual vs uso real + instrucciones | — |

Destructivas / requieren confirmación (dry-run por defecto):

| Tool | Descripción | Parámetros |
|------|-------------|------------|
| `mac_clean_batch` | Borra items safe_to_delete en varios dominios | `domains` (opcional), `dry_run` (default `true`) |
| `mac_docker_cleanup` | Elimina recursos Docker excepto los de `keep` | `keep` (requerido), `dry_run` (default `true`) |
| `mac_docker_backup` | Dump de BD de un contenedor (pg_dumpall/mysqldump) | `container` (requerido) |

> Los agentes `mac-ops` y `kirocrew` solo auto-aprueban las 8 tools de solo lectura. Las 3 destructivas requieren confirmación explícita del usuario antes de ejecutarse con `dry_run=false`.

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

> Si instalaste con `go install`, `toolkit` estará en `$(go env GOPATH)/bin`
> (asegúrate de tenerlo en el `PATH`). Alternativamente usa la ruta absoluta al
> binario, p. ej. `/usr/local/bin/toolkit` o `./bin/toolkit`.

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
`claude mcp add mac-toolkit -- toolkit mcp`.

Guía completa por cliente: **[docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)**.

### Ejemplos desde chat

```
> ¿cómo está mi mac?          → mac_system + mac_battery + mac_processes
> analiza el disco             → mac_analyze (todos los dominios)
> qué puedo limpiar            → mac_clean_preview
> revisa los caches de browser → mac_analyze domain=browser
> revisa la red                → mac_network
```

## Dependencias

- **cobra v1.8.1** — CLI framework
- **go-sdk v1.7.0** — MCP server (Model Context Protocol oficial)
- Go stdlib para todo lo demás (exec.Command, filepath.WalkDir, goroutines, net/http)

## Desarrollo

```bash
make all        # lint + test + build
make test       # go test ./...
make build      # go build -o bin/toolkit
make lint       # go vet ./...
```

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
# 19 tests: core (models, runner), analyzer (helpers, registry), cleaner (blacklist, delete)
```

---

*Mac Toolkit v1.0 — Go CLI + MCP para macOS • [arheanja-ops](https://github.com/arheanja-ops)*

Ver [TOOLKIT_GUIDE.md](TOOLKIT_GUIDE.md) para documentación completa: configuración MCP en Kiro Desktop/CLI, herramientas disponibles, seguridad, skills, y guía de extensibilidad.
