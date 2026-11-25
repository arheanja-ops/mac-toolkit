# Proyecto Mac Toolkit — Bitácora histórica

> **Nota:** esta es una bitácora histórica y resumida. El estado vigente está en
> README.md, CHANGELOG.md y `docs/SESSION_NEXT_STEPS.md`. Se eliminaron las secciones
> que describían configuraciones de entorno ajenas al proyecto.

## Qué se construyó al inicio

- **CLI en Go** (single binary, Cobra) migrada desde un prototipo en Python.
- **11 analyzers**: disk, ollama, docker, browser, logs, downloads, appsupport, repos,
  dev_caches, xcode, trash.
- **Monitors** iniciales: battery (ioreg/pmset), system (vm_stat/top/system_profiler),
  processes (ps), network (netstat/airport).
- **Servidor MCP nativo** (`mac-toolkit mcp`) sobre el SDK oficial de Go, con las
  primeras 7 tools de solo lectura y preview.
- **Integración con Kiro**: 2 agentes, skills y un steering file.

## Archivos clave

- `cmd/mcp.go` — servidor MCP.
- `.kiro/agents/mac-ops.json` y `.kiro/agents/mac-toolkit-dev.json` — agentes de operaciones y desarrollo.
- `docs/MCP_SETUP.md` — guía de conexión a Kiro Crew, Kiro Desktop y Kiro CLI.
- `docs/MCP_SERVERS_SETUP.md` — guía de otros servidores MCP de uso general.
- `TOOLKIT_GUIDE.md` y `README.md` — documentación del proyecto.

## Patrón para agregar un MCP en Kiro Crew

1. Kiro Crew → Agent Capabilities → Connections → MCP Servers → Add Custom.
2. Pegar el JSON → Apply.
3. Kiro Crew no hereda el `PATH` del shell: usar rutas absolutas si el comando no está
   en un directorio estándar.
4. Si `npx` da timeout, instalar el paquete globalmente y usar `node` directo.

---

## Línea de tiempo v1.2.0 → v1.7.1

Nota de cierre. Estado vigente:
README.md, CHANGELOG.md y `docs/SESSION_NEXT_STEPS.md`.

| Versión | Fecha (tag) | Hito |
|---------|-------------|------|
| v1.2.0 | 2026-09-08 | Banner, menú agrupado, reporter con barras; comando `mac-toolkit` |
| v1.3.0 | 2026-09-21 | TUI estilo k9s (dos paneles, header en vivo) |
| v1.3.1 | 2026-09-21 | Bienvenida, acción Salir, total reclaimable sin `disk` |
| v1.3.2 | 2026-09-21 | Fix de timeout del analyzer `repos` |
| v1.4.0 | 2026-11-01 | `doctor` (Groq + heurístico) y tool MCP `mac_doctor` (12 tools) |
| v1.5.0 | 2026-11-01 | Rediseño del TUI, confirmación final `si` en `clean`, `docs/SAFETY.md` |
| v1.6.0 | 2026-11-01 | TUI-2: clean interactivo con gate `si` |
| v1.6.1 | 2026-11-02 | TUI responsivo en 80x24 |
| v1.7.0 | 2026-11-20 | TUI-3: panel Doctor |
| v1.7.1 | 2026-11-23 | Fix térmico (Normal/Throttled/Warning/Unknown) y `--version` con `go install` |

Hoy el servidor MCP expone **12 tools**.
