# TUI-4 — Vistas dedicadas de Memory y CPU — Design Spec

**Date:** 2026-11-23
**Status:** Implemented (v1.8.0)
**Scope:** mac-toolkit CLI (`internal/monitor/`, `internal/tui/`, `internal/ai/`, `cmd/monitors.go`)
**Roadmap:** sigue a [TUI-3](2026-11-02-tui-doctor.md). Solo lectura; sin tools MCP nuevas.

## Problem

`system` y el header muestran un único porcentaje de memoria y de CPU. Ese
porcentaje no distingue una Mac con RAM llena de caché (inofensiva) de una que
pagina a disco, ni una CPU ocupada de una saturada. Además la cifra de memoria
usada (`active+wired+compressed`) no coincide con la de Monitor de Actividad, así
que el usuario no puede contrastarla. Faltan vistas que respondan "¿qué está
pasando y qué consume?" sin salir de la TUI.

## Features

### F1: Monitores `memory` y `cpu` (`internal/monitor`)

- **MemoryMonitor** (`memory`): `ram_total_gb`, `used_gb`/`used_pct`, desglose
  (`app`, `wired`, `active`, `inactive`, `compressed`, `free`, `file_cache`),
  `pressure_level` (+ `pressure_code`) y `free_pct`, swap (`swap_total_mb`,
  `swap_used_mb`), tráfico acumulado (`swapins_gb`, `swapouts_gb`), `uptime_hours`
  y `top_memory` (top 5 por RSS **agregado por aplicación**).
- **CPUMonitor** (`cpu`): `user_pct`/`sys_pct`/`idle_pct` de la **segunda** muestra
  de `top -l 2 -n 0 -s 1`, `load1/5/15`, `cores_total`, `perf_cores`/`eff_cores`
  (si existen), `load_per_core` (load1/núcleos), `load5_per_core`, `load_verdict`,
  `thermal` (reutiliza `parseThermal`) y `top_cpu` (top 5 por %CPU por aplicación).
- Los comandos externos pasan por `runCmd`, una variable inyectable: los tests usan
  salidas sintéticas y nunca la máquina real.
- Las apps se agregan por nombre: la ruta con `/X.app/` da `X` (el bundle más
  externo, así los helpers suman a su app); si no, el basename. **Sin PIDs ni rutas.**

### F2: Definición única de "memoria usada"

`Memory Used = App Memory + Wired + Compressed`, con
`App Memory = Anonymous pages − Pages purgeable` (vm_stat, tamaño de página real).
El helper `readMemUsage` es la única fuente: `SystemMonitor` (`memory_percent`,
`memory_used_gb`), el header de la TUI, la vista Memory y el Doctor leen el mismo
cálculo. El total es `hw.memsize` (fallback: suma de páginas).

### F3: Clasificación pura con umbrales (`internal/monitor/classify.go`)

| Función | Regla |
|---------|-------|
| `ClassifyMemoryPressure(code)` | 1 Normal · 2 Warning · 4 Critical · otro/fallo → Unknown |
| `HeavyPaging(...)` | swap usado > 80 % del total **y** swapouts acumulados > 2× RAM |
| `ClassifyCPU(load5, cores)` / `ClassifyLoadRatio` | load5/núcleos: < 1 Normal · 1–2 Alta (ambos incl.) · > 2 Saturada |
| `HighKernelTime(sys, user)` | sys > 40 % y sys > user |

Los textos sobre causas son prudentes ("puede deberse a paginación o a agentes de
seguridad"); nunca afirman una causa.

### F4: CLI

`mac-toolkit memory` y `mac-toolkit cpu`, con el patrón de `battery/system/…`:
`Display()` con barras y colores de `core` y línea final `✓ Listo (<cmd>)`. Sin
`--json`, como los demás monitores.

### F5: TUI

- Acciones `Memory` y `CPU` en el grupo Monitors, tras System.
- Atajos globales `m` (Memory) y `u` (CPU) en modo normal (no actúan en filtro ni en
  los modos clean/modal). `r` refresca la vista abierta: `doctorOpen` se generaliza
  a `openView` (Doctor/Memory/CPU). Refresco **solo manual**.
- Corre en `tea.Cmd` con spinner (CPU tarda ~2 s por `top`). Mientras corre, `m`/`u`/`d`/`r`
  se ignoran.
- Render responsivo hasta 80x24 con `usableWidth`, `truncateToWidth`, `padLines` y la
  paleta de `styles.go`; barras propias con lipgloss (no `core.ProgressBar`).
- Footer: `m memory · u cpu`; en modo compacto (< 90 col) se abrevia (`p prev`, `m mem`)
  y se omite `/ filtrar` para no truncar `q salir`.

### F6: Doctor

`AISnapshot` suma solo números: `load_per_core` (load5/núcleos), `cpu_user_pct`,
`cpu_sys_pct`, `swap_thrash` (bool) y `memory_pressure_level` (código). Reglas nuevas
en PERFORMANCE: **CPU saturada**, **Tiempo de kernel elevado**, **Paginación intensa**,
con acción sugerida prudente. `collect.go` copia de los monitores solo una lista
permitida de claves; las listas de apps nunca entran al snapshot.

## Reutilización

| Necesidad | Se reutiliza |
|-----------|--------------|
| Estado térmico | `parseThermal`, `IsThermalAlert` |
| Contadores vm_stat | `vmStatVal` |
| Layout TUI | `usableWidth`, `truncateToWidth`, `padLines`, `padCell`, `wrapWords`, `rule` |
| Paleta | `styles.go` |
| Snapshot Doctor | `ai.BuildSnapshot`, `ai.BuildDoctorSnapshot` |
| Patrón de comandos | `cmd/monitors.go` |

## Data model

```
// internal/monitor
var runCmd func(name string, args ...string) ([]byte, error) // inyectable
type AppUsage struct{ Name string; MemMB, CPUPct float64; Procs int } // sin PID/ruta
func ClassifyMemoryPressure(int) string; func HeavyPaging(...) bool
func ClassifyCPU(load5 float64, cores int) string; func HighKernelTime(sys, user float64) bool

// internal/tui
type viewID int // viewNone | viewDoctor | viewMemory | viewCPU  (model.openView)
func formatMemory(d map[string]any, width int) string
func formatCPU(d map[string]any, width int) string

// internal/ai (AISnapshot)
LoadPerCore, CPUUserPct, CPUSysPct float64; SwapThrash bool; MemoryPressureLevel int
```

## Non-goals

- Tools MCP nuevas (`mac_memory`/`mac_cpu`).
- Auto-refresh de las vistas.
- Matar procesos o cualquier acción que modifique el sistema.
- Histórico de métricas o gráficos temporales.

## Testing

- Parsers con salidas sintéticas de `vm_stat` (páginas de 16384 y 4096 bytes),
  `top`, `sysctl` y `memory_pressure`; comando que falla → `Unknown`/clave ausente,
  sin pánico.
- Agregación por aplicación (helpers dentro de su bundle, basename, líneas inválidas).
- Clasificadores con tabla de umbrales, bordes incluidos.
- `SystemMonitor` y `MemoryMonitor` devuelven la misma memoria usada.
- TUI: `m`/`u`/`r`, regresión de `a/p/c/d/r`, atajos ignorados en modos y filtro,
  `TestViewFitsEightyByTwentyFour` con los paneles Memory y CPU.
- Doctor: las 3 reglas nuevas, silencio en un sistema sano, y privacidad del snapshot
  (solo números/booleanos; sin rutas, PIDs ni nombres).

## Seguridad

Todo es de **solo lectura**: se ejecutan `vm_stat`, `sysctl`, `memory_pressure`,
`top`, `ps` y `pmset` con `exec.Command` sin shell. No hay señales a procesos, ni
escrituras, ni teclas nuevas que muten. Las vistas muestran nombres de aplicación
(locales) pero ninguna ruta ni PID; el Doctor solo recibe números.

## Entrega

PR único; bump **MINOR** (`v1.8.0`). Flujo: [RELEASE_WORKFLOW.md](../RELEASE_WORKFLOW.md).
