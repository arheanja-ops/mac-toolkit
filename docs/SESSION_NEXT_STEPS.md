# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-25
**Repo:** `arheanja-ops/mac-toolkit` · rama `main` · último release **v1.9.0**

## ✅ Resuelto en v1.9.0

- **Menú por categorías** y vistas de Memory, CPU, Processes y Reports (solo lectura;
  `Save report` escribe solo en la carpeta de reportes).
- **Processes en la TUI**: tablas por ancho en vez de datos crudos.
- **Reportes**: carpeta fija `os.UserConfigDir()/mac-toolkit/reports` (también el audit);
  `Last report` funciona desde cualquier directorio y, sin reportes, guía al usuario.
- **CPU% del header y de `system`**: intervalo de 1 s de `iostat` (~0 CPU, antes ~1.4 s
  por lectura con `top`). Veredicto "Carga alta con CPU libre" cuando idle > 50 %.
  Spec: `docs/specs/2026-11-25-tui-categories.md`.

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
4. **Agentes `.kiro/agents/*.json`:** sus prompts y `allowedTools` deben conocer el
   parámetro `confirm_plan` y el opt-in `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` de las tools
   MCP destructivas (v1.7.2). Va en el PR de agentes del punto 2.

## 🎯 Siguiente versión: v1.10.0 — "modo escucha" (solo propuesta, sin implementar)

Monitoreo constante mientras la herramienta está abierta, para avisar antes de que el
usuario lo note. Debe diseñarse en su propia spec antes de escribir código:

- **Monitoreo continuo** dentro de la TUI abierta (no un daemon): un muestreo
  periódico de CPU, memoria/presión, swap y térmico.
- **Alertas por umbrales sostenidos**: una alerta solo se dispara si la condición
  se mantiene N muestras seguidas (p. ej. presión Critical o carga alta con idle bajo),
  no por un pico aislado; reutiliza los clasificadores de `internal/monitor/classify.go`.
- **Notificaciones de macOS** (`osascript`/Notification Center) además del aviso en el header.
- **Anti-spam**: enfriamiento por tipo de alerta, histéresis para el aviso de "recuperado"
  y un máximo de avisos por hora.
- **Muestreo liviano**: intervalo configurable, lecturas baratas (`iostat`, `sysctl`,
  `vm_stat`), nunca `top`/`system_profiler` en el bucle; el costo debe medirse y
  documentarse como el del header.
- **Registro de historial**: serie corta de métricas y alertas en la carpeta del
  toolkit (rotación por tamaño), para consultar qué pasó mientras no se miraba.
- Preguntas abiertas: ¿opt-in con una tecla o flag?, ¿umbrales configurables?, ¿qué
  privacidad tiene el historial (solo números, sin nombres de procesos)?

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
| v1.9.0 | 2026-11-25 | Menú por categorías, vistas de Processes y Reports, reportes en carpeta fija, CPU% liviano |

## 🔒 Seguridad (ver docs/SAFETY.md)

`doctor`, `analyze` y monitores (incl. Memory, CPU y Processes): solo lectura (`Save report` solo escribe en la carpeta de reportes). `clean`: dry-run por defecto →
`--execute` → aprobación → confirmación "si". TUI: solo borra items `safe`, con modal
"si", blacklist y audit log. MCP: tools destructivas con `dry_run=true` por defecto, opt-in del servidor y `confirm_plan`. Docker cleanup: plan + "si" + audit.

## 📋 Flujo (recordatorio)

rama → PR → CI verde → merge squash → (si hay versión) tag vX.Y.Z → release →
`go install` (el repo es público; alternativa: binario del release + checksum). Los PRs solo de docs no llevan tag. Nunca a main directo.
`git add` con rutas explícitas. Detalle: docs/RELEASE_WORKFLOW.md.
