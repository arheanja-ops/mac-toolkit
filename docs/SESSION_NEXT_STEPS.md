# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-23
**Repo:** `arheanja-ops/mac-toolkit` · rama `main` · último release **v1.8.0**

## ✅ Resuelto en v1.8.0

- **Vistas Memory y CPU (solo lectura):** comandos `memory`/`cpu`, acciones en la TUI,
  atajos `m`/`u` y `r` genérico (`openView`). Apps agregadas por aplicación, sin PIDs
  ni rutas.
- **Una sola definición de memoria usada** (Monitor de Actividad): `system`, header,
  Memory y Doctor muestran la misma cifra.
- **Doctor:** insights de CPU saturada, tiempo de kernel elevado y paginación intensa.
  Spec: `docs/specs/2026-11-23-tui-memory-cpu.md`.

## ✅ Resuelto en v1.7.2

- **Gates de seguridad en borrados (SAFETY.md g):** `docker cleanup --execute` ahora
  muestra el plan, exige "si" y escribe audit log; los errores de docker se informan.
  Las tools MCP destructivas exigen opt-in (`MAC_TOOLKIT_MCP_ALLOW_DELETE=1`) y
  `confirm_plan` = `plan_digest`; `mac_clean_batch` y `mac_docker_cleanup` escriben
  audit log.

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
4. **CPU del header y de `system`:** `cpu_percent` sale de la primera muestra de
   `top -l 1`, que es acumulada; la vista CPU usa la segunda muestra. Valorar que
   `system`/header usen el mismo cálculo que `CPUMonitor`.
5. **Agentes `.kiro/agents/*.json`:** sus prompts y `allowedTools` deben conocer el
   parámetro `confirm_plan` y el opt-in `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` de las tools
   MCP destructivas (v1.7.2). Va en el PR de agentes del punto 2.

## 🎯 Roadmap (por blast radius creciente)

- **TUI-2.1:** limpiar `warn`/`danger` desde la TUI con confirmación por item.
- **Docker en la TUI:** backup / cleanup / compact como acciones.
- **Fase 2 IA:** `doctor` agente (function-calling).
- **Fase 3 IA:** `optimize --safe`.
- **MCP `mac_memory`/`mac_cpu`:** descartado por ahora (decisión de producto).

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
| v1.7.2 | 2026-11-23 | Gates de seguridad: Docker cleanup (plan + "si" + audit) y MCP destructivo (opt-in + `confirm_plan` + audit) |
| v1.8.0 | 2026-11-23 | Vistas Memory y CPU (`m`/`u`/`r`), memoria usada como Monitor de Actividad, insights nuevos en `doctor` |

## 🔒 Seguridad (ver docs/SAFETY.md)

`doctor`, `analyze` y monitores (incl. Memory y CPU): solo lectura. `clean`: dry-run por defecto →
`--execute` → aprobación → confirmación "si". TUI: solo borra items `safe`, con modal
"si", blacklist y audit log. MCP: tools destructivas con `dry_run=true` por defecto, opt-in del servidor y `confirm_plan`. Docker cleanup: plan + "si" + audit.

## 📋 Flujo (recordatorio)

rama → PR → CI verde → merge squash → (si hay versión) tag vX.Y.Z → release →
`go install @vX.Y.Z`. Los PRs solo de docs no llevan tag. Nunca a main directo.
`git add` con rutas explícitas. Detalle: docs/RELEASE_WORKFLOW.md.
