# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-23
**Repo:** `arheanja-ops/mac-toolkit` · rama `main` · último release **v1.7.1**

## ✅ Resuelto en v1.7.1

- **Estado térmico:** el falso positivo "Warning" está corregido. `parseThermal`
  (`internal/monitor/thermal.go`) clasifica `pmset -g therm` en Normal / Throttled /
  Warning / Unknown y `IsThermalAlert` solo trata Throttled y Warning como alerta.
- **`--version`:** con `go install ...@vX.Y.Z` ya muestra la versión (se lee de
  `runtime/debug.ReadBuildInfo()`); commit y fecha reales solo vienen en los
  binarios del release (`none`/`unknown` con `go install`).

## ▶ RETOMAR AQUÍ (pendientes reales)

1. **Higiene `gofmt`:** `gofmt -l .` marca `internal/analyzer/*`,
   `internal/docker/ops.go` e `internal/tui/helpers_test.go`. PR de formato puro
   (sin cambios de lógica), aparte de cualquier feature.
2. **`.kiro/agents/*.json` y `*.bak`:** hay cambios locales del IDE sin commitear.
   Decidir si se commitean (en su propio PR) o se descartan. Nunca mezclarlos con
   features ni con docs. Nota: el prompt de `mac-ops` sigue diciendo 11 tools (hoy son
   12, falta `mac_doctor`) y su lista `allowedTools` no incluye `mac_doctor`.
3. **Versión del servidor MCP:** `cmd/mcp.go` anuncia `Version: "1.0.0"` fija en
   `mcp.Implementation`; conviene que siga la versión del binario.
4. **Huecos de seguridad documentados:** `docker cleanup --execute` y
   `mac_docker_cleanup` no pasan por el gate `si`, la blacklist ni el audit log
   (ver `docs/SAFETY.md`, apartado g); `mac_clean_batch` no escribe audit log.

## 🎯 Roadmap (por blast radius creciente)

- **TUI-2.1:** limpiar `warn`/`danger` desde la TUI con confirmación por item.
- **Docker en la TUI:** backup / cleanup / compact como acciones.
- **Fase 2 IA:** `doctor` agente (function-calling).
- **Fase 3 IA:** `optimize --safe`.

## 💡 Propuesta (NO implementada): v1.8.0 "Presión del sistema"

Hoy `system` reporta CPU, memoria, swap y estado térmico, pero no distingue una
máquina saturada de una ocupada. Idea para un MINOR futuro, solo lectura:

- **Load average vs núcleos** (`sysctl vm.loadavg` / `hw.ncpu`): carga sostenida
  muy por encima del número de núcleos.
- **Swap usado y compresor** (`sysctl vm.swapusage`, `vm_stat`): cuánto del swap y
  de la memoria comprimida está en uso.
- **Swap in/out** (`vm_stat`, páginas swapped in/out desde el arranque): detectar
  thrashing.
- Exponerlo en `system`, en el header/panel de la TUI, en `mac_system` y como
  insight del `doctor` (agregados, sin PII), con tests de tabla sobre salidas reales
  de los comandos, como se hizo con el parser térmico.

Antes de implementar: spec en `docs/specs/` (requirements → design → tasks) y
confirmar los nombres exactos de los contadores con los comandos reales.

## ✅ Entregado y en producción

| Versión | Fecha (tag) | Qué entregó |
|---------|-------------|-------------|
| v1.2.0 | 2026-09-08 | UX: banner, menú, presentación de datos, nombre `mac-toolkit` |
| v1.3.0–1.3.2 | 2026-09-21 | TUI k9s (2 paneles, header vivo) + bienvenida/Salir + fix total reclaimable + fix timeout `repos` |
| v1.4.0 | 2026-11-01 | Comando `doctor` con IA (Groq) + fallback heurístico + tool MCP `mac_doctor` |
| v1.5.0 | 2026-11-01 | Rediseño del TUI + gate de confirmación final en `clean` + `docs/SAFETY.md` |
| v1.6.0 | 2026-11-01 | TUI-2: clean interactivo (checkboxes + modal "si", solo items safe) |
| v1.6.1 | 2026-11-02 | TUI responsivo en 80 columnas + feedback "✓ Listo" |
| v1.7.0 | 2026-11-20 | TUI-3: panel Doctor (`d`, `r` refresca) + `internal/ai/collect.go` |
| v1.7.1 | 2026-11-23 | Fix falso positivo térmico + `--version` con `go install` |

## 🔒 Seguridad (ver docs/SAFETY.md)

`doctor`, `analyze` y monitores: solo lectura. `clean`: dry-run por defecto →
`--execute` → aprobación → confirmación "si". TUI: solo borra items `safe`, con modal
"si", blacklist y audit log. MCP: tools destructivas con `dry_run=true` por defecto.

## 📋 Flujo (recordatorio)

rama → PR → CI verde → merge squash → (si hay versión) tag vX.Y.Z → release →
`go install @vX.Y.Z`. Los PRs solo de docs no llevan tag. Nunca a main directo.
`git add` con rutas explícitas. Detalle: docs/RELEASE_WORKFLOW.md.
