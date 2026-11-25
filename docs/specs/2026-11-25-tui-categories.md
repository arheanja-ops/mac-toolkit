# TUI-5 — Menú por categorías, vistas de procesos y reportes — Design Spec

**Date:** 2026-11-25
**Status:** Implemented (v1.9.0)
**Scope:** mac-toolkit CLI (`internal/monitor/`, `internal/core/`, `internal/reporter/`, `internal/tui/`, `internal/ai/`, `cmd/`)
**Roadmap:** sigue a [TUI-4](2026-11-23-tui-memory-cpu.md). Solo lectura salvo `Save report`.

## Problem

1. El menú plano de la TUI mezclaba acciones sin orden y Processes mostraba datos
   crudos del monitor (structs Go) en vez de una tabla.
2. `Last report` decía "no reports found": los reportes se guardaban en `reports/`
   relativo al directorio de trabajo, así que cambiaba según desde dónde se lanzara
   la herramienta.
3. El `cpu_percent` del header y de `system` salía de la primera muestra de
   `top -l 1` (promedio desde el arranque) y costaba ~1.4 s de CPU por lectura.
4. Con carga alta y CPU casi ociosa el veredicto decía "Saturada", aunque la CPU no
   era el límite (la carga también cuenta procesos esperando memoria o disco).

## Features

### F1: Menú por categorías

HEALTH (Doctor) · DISK (Analyze, Status) · CLEAN (Preview, Select & clean) ·
MEMORY (Overview, Swap & paging, Top apps) · CPU (Overview, Load, Top apps, Thermal) ·
PROCESSES (Top by CPU, Top by memory, By app, Summary) · SYSTEM (Battery, System info,
Network) · REPORTS (Last report, Saved reports, Save report) · SESSION (Quit).

- El estado (ítems, cursor, filtro) sigue en `bubbles/list`; el dibujo lo hace
  `renderMenu`: ventana que sigue al cursor, con la cabecera de su categoría fija
  en la primera línea, y separadores solo si caben. El cursor salta cabeceras
  (`↑↓`, `Home/End`, `PgUp/PgDn`). Cabe en 80x24 con el cursor en cualquier ítem.
- Atajos: `a` Analyze · `p` Preview · `c` Select & clean · `d` Doctor · `m` Memory ·
  `u` CPU · `r` refresca la vista abierta · `/` filtra · `q` sale. Abrir una vista por
  atajo mueve el cursor a su entrada. El footer pasa a su forma compacta cuando el
  texto completo no cabe en el ancho (así `q salir` nunca se trunca).

### F2: Vistas de Processes, Memory y CPU

- **Processes**: Top by CPU / by memory (PID, %CPU, %MEM, RSS legible, nombre —
  basename, nunca ruta), By app (procesos, %CPU, RAM, agregado por aplicación) y
  Summary (por estado + presión de memoria). `ProcInfo` y `ProcSummary` son tipos
  exportados; la TUI recibe valores tipados y formatea tablas (las columnas se
  degradan en paneles estrechos: primero `%MEM`, luego `RSS`).
- **Memory**: Overview, Swap & paging, Top apps. **CPU**: Overview, Load, Top apps,
  Thermal. Todas leen del mismo monitor que el comando CLI.
- Cada línea se corta al ancho (`lipgloss.Width`); ninguna vista imprime `{}`/`map[`.

### F3: Carpeta canónica de reportes (`internal/core/reports.go`)

- `core.ReportsDir()` = `os.UserConfigDir()/mac-toolkit/reports` (en macOS
  `~/Library/Application Support/mac-toolkit/reports`), siempre absoluta. Un único
  helper la usa `report`, `analyze --save`, `full`, los reporters, el audit de
  `clean` y la TUI. Ya no depende del directorio de trabajo.
- `ListReports`/`LatestReport` devuelven los análisis (`analysis_<fecha_hora>/report.md`),
  más recientes primero; los audits (`cleanup_*`) se omiten. Una carpeta inexistente
  no es un error.
- `reporter.SaveAnalysis` es el único camino de guardado (markdown + JSON).
  `ReportsDirOverride` existe solo para tests.
- `Last report` y `Saved reports` sin reportes muestran un mensaje amistoso (cómo crear
  el primero y la carpeta, con `~` en vez del home). `Save report` analiza y guarda
  en esa carpeta y muestra la ruta.

### F4: CPU% correcto y liviano

- `cpu_percent` = user+sys de `iostat -c 2 -w 1`: la segunda línea es el intervalo
  de 1 s (la primera es el acumulado desde el arranque). Cuesta ~0 CPU frente a
  ~1.4 s de `top -l 1`. Las columnas se localizan desde el final de la línea (el
  número de discos varía). Si `iostat` falla, la clave se omite (no se inventa 0).
- `SystemMonitor{Light: true}` salta `system_profiler` para el header de la TUI.
  `CPUMonitor` comparte la misma lectura (`user_pct`/`sys_pct`/`idle_pct`).

### F5: Veredicto de carga con idle

`ClassifyLoad(ratio, idlePct)`: < 1 Normal; desde 1, si idle > 50 % →
**"Carga alta con CPU libre: procesos esperando memoria o disco"**; si no, 1–2 Alta,
> 2 Saturada. `idlePct < 0` (desconocido) conserva la regla solo por carga. El
Doctor suma un insight "Carga alta con CPU libre" (severidad media); el snapshot de IA
añade solo `cpu_idle_pct` (número), sin nombres, rutas ni PIDs.

## Data model

```
// internal/core
var ReportsDirOverride string                  // solo tests
func ReportsDir() string; NewReportDir(prefix string) string
type ReportEntry struct{ Name, Path string; Size int64; ModTime time.Time }
func ListReports() ([]ReportEntry, error); LatestReport() (ReportEntry, bool, error)

// internal/reporter
func SaveAnalysis(results []core.AnalysisResult, quiet bool) (dir string, err error)

// internal/monitor
type ProcInfo struct{ PID int; CPU, Mem float64; RSSKB int64; Name string }
type ProcSummary struct{ Total, Running, Sleeping, Stopped, Zombie, IOWait int }
func ClassifyLoad(ratio, idlePct float64) string
type SystemMonitor struct{ Light bool }
```

## Non-goals

- Modo escucha / alertas (ver `docs/SESSION_NEXT_STEPS.md`, v1.10.0).
- Contar hilos (costaría un `ps` extra por hilo).
- Matar procesos o cualquier acción que cambie el sistema.
- Migrar reportes antiguos de `reports/` relativos al directorio de trabajo.

## Testing

- `internal/monitor`: parser de `iostat` con salida sintética (1 y 2 discos), degradación
  si falla, `ClassifyCPU` con tabla (carga alta + idle alto, sistema sano, idle
  desconocido), agregación y resumen de procesos. Todo con `runCmd` inyectable.
- `internal/tui` (`categories_test.go`): cada vista nueva a 40/50/60/80 columnas sin
  líneas más anchas ni `{}`; navegación en 80x24 (primer y último ítem, `View()` ≤ 24
  líneas y ≤ 80 columnas); `a/p/c/d/m/u/r` y regresiones; modos clean/modal; reporte
  encontrado tras `t.Chdir`; mensaje amistoso sin reportes. `TestMain` fija
  `core.ReportsDirOverride` a un directorio temporal: ningún test toca la máquina real.
- `internal/reporter`: `SaveAnalysis` escribe en la carpeta canónica desde otro cwd.
- `internal/ai`: `cpu_idle_pct` es numérico y el snapshot no filtra nombres ni PIDs.

## Seguridad

Todo es de **solo lectura** salvo `Save report`, que únicamente escribe `report.md` y
`report.json` dentro de la carpeta de reportes del toolkit; no borra ni cambia nada
fuera de ella. El audit de `clean` pasa a la misma carpeta
(`cleanup_<fecha_hora>/audit.json`). Las vistas muestran nombres locales, no rutas; el
PID aparece solo en la pantalla del propio usuario y nunca llega al snapshot de IA.

## Entrega

PR único; bump **MINOR** (`v1.9.0`). Flujo: [RELEASE_WORKFLOW.md](../RELEASE_WORKFLOW.md).
