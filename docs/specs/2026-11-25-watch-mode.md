# Modo escucha (watch) — Design Spec

**Date:** 2026-11-25
**Status:** Implemented (v1.11.0). Approved con: umbrales sostenidos, anti-spam, notificaciones macOS, historial 7 días
**Scope:** `internal/watch/` (motor nuevo), TUI, CLI (`watch`, `history`), carpeta canónica de datos.
**Versión objetivo:** v1.11.0 (MINOR) — después de Report v2 (v1.10.0); `history` reutiliza su estilo de tablas.

## Problem

La herramienta solo ve el instante en que se consulta. Cuando el Mac se pone lento o se cuelga,
no hay registro de qué pasó antes ni aviso mientras ocurre: hoy no se pudo reconstruir qué pasó
entre las 08:00 y las 10:20. Se necesita un monitoreo continuo y liviano mientras la herramienta
está abierta, que alerte cuando un problema se sostiene y deje historial consultable.

## Features

### F1 — Motor de muestreo (`internal/watch`)
- Un `Sampler` toma una muestra cada **15 s** con fuentes livianas, sin `top`:
  - `sysctl` para carga (1/5/15), swap usado/total, presión de memoria
    (`kern.memorystatus_vm_pressure_level`) y núcleos;
  - `vm_stat` para contadores de swapins/swapouts y pageins/pageouts (se guarda el delta → tasa);
  - `iostat -c 2 -w 1` para user/sys/idle de CPU (segunda muestra);
  - `statfs` (llamada directa, sin exec) para disco libre;
  - `pmset -g therm` cada **60 s** (cambia poco).
- Corre en una goroutine con `context` cancelable; se detiene al cerrar la TUI o con Ctrl+C.
- Presupuesto de costo: **< 1 % de CPU promedio** y < 30 MB de RAM. Se mide y documenta.
- Ante fallo de una fuente: esa métrica queda vacía en la muestra; el motor no se detiene.

### F2 — Reglas con umbral sostenido
Una alerta se dispara solo si la condición se cumple en una ventana (no por un pico):

| Regla | Condición | Ventana | Severidad |
|---|---|---|---|
| Presión de memoria | nivel Warning / Critical | 2 min | warn / critical |
| Swap casi lleno | swap usado > 90 % | 2 min | warn |
| Paginación intensa | tasa de swapout > 20 MB/s | 2 min | critical |
| CPU saturada | load5 por núcleo > 2 **y** idle < 20 % | 3 min | warn |
| Carga con CPU libre | load5 por núcleo > 2 **y** idle > 50 % | 3 min | info (espera de memoria/disco) |
| Disco casi lleno | libre < 10 GB o < 5 % | inmediata | warn |
| Térmico | Throttled / Warning (`IsThermalAlert`) | 1 min | warn |

- Umbrales como constantes en un solo lugar, con su porqué documentado.
- Al normalizarse (ventana completa por debajo), la alerta pasa a "resuelta".

### F3 — Anti-spam
- Cooldown por regla: no repite la misma alerta antes de **15 min**.
- Solo re-notifica antes del cooldown si **escala** de severidad (warn → critical).
- Tope global: **6 notificaciones por hora**; el resto queda en el historial sin notificar.
- Notificación de "resuelto" solo para alertas critical.

### F4 — Notificaciones de macOS
- `osascript -e 'display notification "<texto>" with title "mac-toolkit" subtitle "<regla>"'`
  vía `exec.Command` con argumentos (sin shell) y escapado de comillas del texto.
- Texto corto con la cifra y la acción sugerida, en tono prudente
  (p. ej. "Swap al 94 % hace 2 min. Cierra apps que no uses.").
- Sin rutas ni PIDs. Puede incluir el nombre de la app que más consume.
- Activables/desactivables: `--no-notify` en CLI y tecla `n` en la TUI (estado visible en el header).

### F5 — Integración con la TUI
- El motor arranca solo al abrir la TUI (si F4 está activo, notifica).
- Header: indicador de escucha (`● escuchando`) y contador de alertas activas (`⚠ 2`).
- Nueva categoría **ALERTS** en el menú: *Active alerts* (regla, severidad, desde cuándo, cifra
  actual) e *History* (últimas 24 h resumidas por hora).
- Atajos: `n` alterna notificaciones; `w` pausa/reanuda la escucha.
- El header en vivo reutiliza la última muestra del motor en lugar de leer por su cuenta
  (una sola fuente de lectura, menos carga).

### F6 — CLI
- `mac-toolkit watch` — mismo motor sin TUI, para dejarlo en una pestaña de terminal;
  imprime una línea por alerta y un resumen breve cada 15 min. `--no-notify`.
- `mac-toolkit history [--since HH:MM|YYYY-MM-DDTHH:MM] [--until …]` — resumen del rango:
  mín/promedio/p95/máx de memoria usada, swap, tasa de paginación, carga por núcleo, idle y disco;
  lista de alertas con hora de inicio y fin; apps que más aparecieron como top. `--json` opcional.

### F7 — Historial
- Archivo JSONL por día en `<carpeta canónica>/history/YYYY-MM-DD.jsonl`
  (`os.UserConfigDir()/mac-toolkit/history`): una línea por muestra (solo números + nombre
  de la app top por CPU y por RAM) y una por evento de alerta.
- Tamaño estimado: ~1–2 MB por día.
- Retención: **7 días**. Al arrancar, borra solo archivos `*.jsonl` de esa carpeta con más de
  7 días (nunca otra cosa; con prueba).

## Reutilización
- `MemoryMonitor`, `CPUMonitor`, `parseThermal`/`IsThermalAlert`, agregación por app (`apps.go`),
  `runCmd` inyectable, helper de carpeta canónica (`internal/core/reports.go`), primitivas de
  layout y paleta de la TUI.

## Data model
- `Sample{Time, Load1, Load5, Load15, Cores, IdlePct, UserPct, SysPct, MemUsedPct, Pressure,
  SwapUsedPct, SwapOutMBps, SwapInMBps, DiskFreeGB, Thermal, TopCPUApp, TopMemApp}`
- `Alert{Rule, Severity, Since, Value, State(active|resolved), Notified}`
- `Event{Time, Kind(alert|resolve), Alert}`

## Non-goals
- Daemon/launchd que corra con la herramienta cerrada (posible fase futura).
- Matar procesos o cualquier acción correctiva automática: el modo escucha es **solo lectura**
  y solo notifica.
- Tool MCP de historial (puede añadirse después).
- Umbrales configurables por archivo (constantes por ahora).

## Testing
- Reglas con series sintéticas: pico corto no alerta; condición sostenida sí; recuperación → resuelta.
- Anti-spam: cooldown, escalado, tope por hora.
- Escapado de texto de la notificación (comillas, saltos de línea) y que no se invoca `osascript`
  con `--no-notify`. Runner inyectable: ningún test notifica de verdad.
- Historial: escritura/lectura JSONL, resumen de rango (`history --since/--until`), retención que
  solo borra `*.jsonl` viejos de su carpeta.
- TUI: categoría ALERTS a 40/50/60/80 columnas y 80x24, atajos `n`/`w` sin romper los existentes.
- Privacidad: muestras e historial sin rutas ni PIDs.
- Costo: benchmark del ciclo de muestreo.

## Seguridad
Solo lectura. Las únicas escrituras son el historial en la carpeta propia de la herramienta y la
poda de sus propios `*.jsonl` con más de 7 días. Actualizar `docs/SAFETY.md`.

## Entrega
Un PR (spec + implementación + docs), bump MINOR **v1.11.0**, flujo de `docs/RELEASE_WORKFLOW.md`.

## Implementación (v1.11.0)

- `internal/monitor/light.go`: `ReadLight(LightOptions)` reutiliza los parsers del monitor.
  Las fuentes opcionales son `pmset` (cada 60 s) y `ps` (cada 30 s; la app top se conserva
  entre lecturas).
- `internal/watch`:
  - `sample.go`: `Sampler` deriva las tasas de swap; el disco se lee con `statfs`.
  - `rules.go`: `Evaluator` con ventanas; un hueco de más de 45 s rompe la ventana.
  - `notify.go`, `history.go` (`Store` con `Append`, `Read` y `Prune`), `summary.go`
    (`Summarize` y `Hourly`) y `engine.go` (`Engine.Step`/`Run`, estado concurrente).
  - `format.go`: salida de terminal.
- TUI (`internal/tui/watch_view.go`): badge en el header, Active alerts, History y teclas
  `n`/`w`. CLI: `cmd/watch.go`, `cmd/history.go` y `--no-notify` en la TUI.
- **Costo medido** (Apple M1 Pro, 2 min de `watch --no-notify`): 0,72 s de CPU en 122 s
  (≈0,6 %), RSS máximo ~20 MB. Ciclo interno sin comandos: ~9 ms (`BenchmarkSamplingCycle`).

