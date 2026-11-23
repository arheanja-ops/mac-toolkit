# TUI-3 — Panel Doctor dentro del TUI — Design Spec

**Date:** 2026-11-02
**Status:** Implemented (v1.7.0)
**Scope:** mac-toolkit CLI (`internal/tui/`, extrae el armado de snapshot a `internal/ai`; reutiliza `ai.Diagnose`)
**Roadmap:** sigue a [TUI-2](2026-11-01-tui-clean.md). Fase TUI-3. Depende de [AI Doctor](2026-09-09-ai-doctor.md).

## Problem

`mac-toolkit doctor` y la tool MCP `mac_doctor` ya producen un `ai.Diagnosis`
priorizado (PERFORMANCE / QUICK WINS / REQUIERE REVISIÓN + contexto de disco),
pero dentro del TUI no existe: hay que salir y correr `doctor`. El atajo `d` quedó
reservado en TUI-2. Además, el armado del snapshot vive en `cmd/doctor.go`
(`buildDoctorSnapshot`, `systemMetrics`, `topCPUProcess`) y es inaccesible desde
`internal/tui/` (el paquete `cmd` importa a `tui`, no al revés), por lo que
reutilizarlo hoy obligaría a duplicar lógica.

TUI-3 añade el panel Doctor, **solo lectura**, y consolida el armado del snapshot en
una función compartida para CLI, MCP y TUI.

## Features

### F1: Acción `Doctor` + atajo `d`

**Panel izquierdo:** nueva acción `Doctor` en el grupo `Disk` (junto a Analyze/Status;
evita un grupo de una sola entrada). **Atajo global:** `d` (solo en `normalMode` y sin
filtro activo, igual que `a`/`p`/`c`) ejecuta la misma acción vía `runAction`.

Constraints:
- Es una acción de solo lectura con `run(width int) (string, error)`; no usa flags
  de control (`quit`/`openClean`).
- No cambia `mode`: el Doctor se muestra en el viewport del panel derecho como
  Analyze/Preview. Las teclas `d`/Enter durante `running` no lanzan una segunda
  ejecución (se ignoran mientras `m.running`).

### F2: Ejecución asíncrona con spinner y motor visible

Behavior:
1. El run se ejecuta en un `tea.Cmd` (goroutine) como el resto de acciones; el
   panel derecho muestra el spinner existente (`m.running`). El loop nunca se
   bloquea, incluida la llamada a Groq (timeout 20 s, ya interno a `ai.Diagnose`).
2. Orquestación: `ai.BuildDoctorSnapshot()` (F5) → `ai.Diagnose(snap, ai.Options{})`.
   Sin `GROQ_API_KEY`, o ante cualquier fallo de Groq, `Diagnose` ya cae al
   heurístico **en silencio**; el TUI no muestra errores de IA.
3. La primera línea del panel indica siempre el motor, con la misma lógica que
   `cmd/doctor.go:engineLabel`: `motor: groq · <model>` o `motor: heurístico`.
4. Mientras corre, `Esc`/`q` siguen funcionando (salir del TUI); la goroutine se
   descarta (su resultado llega a un modelo ya cerrado, sin efecto).

Constraints:
- Sin cache en esta fase: cada ejecución re-analiza (puede tardar varios segundos;
  el spinner lo comunica). Cada pulsación de `d` re-analiza, y `r` refresca
  (re-analiza) estando en el panel Doctor; `r` en cualquier otro contexto no hace
  nada. Ambas se ignoran mientras `m.running`.
- El TUI no pasa `NoAI` (no hay flag); el opt-out sigue siendo no definir la key.

### F3: Formato del panel (responsive, paleta compartida)

```
 motor: heurístico
 ─────────────────────────────────────────────
 ⚡ PERFORMANCE
  • Swap alto — 21.0 GB en uso
    Cierra apps pesadas o reinicia
 🧹 QUICK WINS  (24.8 GB recuperables sin riesgo)
  • Cachés de dev — 5.6 GB
    Presiona c para limpiar
 ⚠ REQUIERE REVISIÓN
  • Application Support — 36 GB
    Revisa manualmente (no borrable aquí)
 ─────────────────────────────────────────────
 Contexto: disco 62% ▰▰▰▰▰▰▱▱▱▱ · 128 GB libres de 460 GB
```

Behavior:
1. Función pura `formatDiagnosis(d ai.Diagnosis, model string, width int) string`
   (en `internal/tui/doctor.go`), análoga a `formatAnalysis`.
2. Secciones PERFORMANCE / QUICK WINS / REQUIERE REVISIÓN (vacías se omiten, como
   `renderGroup`), seguidas de la línea de contexto de disco (se omite si
   `DiskTotalBytes <= 0`; es contexto, nunca suma a recuperable).
3. Cada insight: `• <Title> — <Detail>` con el título coloreado por severidad
   (`high`→`riskDanger`, `medium`→`riskWarn`, `low`→`riskSafe`, mismo mapeo que
   `cmd/doctor.go:severityColor`) y `Action` en una línea atenuada (`textMuted`).
4. Estilos: solo la paleta de `styles.go` (`tableHeaderStyle`, `riskSafe/Warn/Danger`,
   `textMuted`, `ruleStyle`, `rule(w)`). Sin estilos nuevos salvo que falte uno.
5. Responsive: `w := usableWidth(width)`; cada línea (título, detalle, acción,
   contexto) se **envuelve** (word-wrap por celdas con `lipgloss.Width`) o se trunca
   con `truncateToWidth` si no cabe; nunca excede `w`. La barra de disco es **propia del TUI**
   (lipgloss, `proportionBar`; medida con `lipgloss.Width`), de ancho `min(20, w-30)`,
   y se omite bajo 50 columnas, dejando solo el texto del porcentaje.
   El resultado pasa por `padLines` como el resto de acciones.
6. Emoji de cabecera: se mide con `lipgloss.Width` (no `len`); si el terminal es
   estrecho (<50) se omiten los emoji para evitar desfase por celdas ambiguas.

Constraints:
- 80x24 sin desbordar: el viewport ya hace scroll vertical; el límite estricto es
  el ancho. `TestViewFitsEightyByTwentyFour` debe seguir pasando.
- Datos de insights provienen del Diagnosis (agregados); el formateador no consulta
  el filesystem ni añade rutas.

### F4: Puente a la acción (sin romper el gate)

Behavior:
1. Cada Quick Win cuya acción sea de limpieza muestra la línea
   `→ presiona c para limpiar` (texto estático, estilo `textMuted`), en lugar de
   comandos shell. `c` ya abre el modo selección de TUI-2.
2. El pie de QUICK WINS añade una única pista: `[c] limpiar seguros · [p] preview`.

Constraints:
- **El Doctor nunca borra ni ejecuta.** No registra teclas nuevas que mutaten; no
  llama a `cleaner`, no cambia `mode`. El borrado sigue exigiendo F2+F3 de TUI-2
  (selección explícita + "si", blacklist, audit log).
- Si `ReclaimableSafeBytes == 0`, no se muestra la pista `c`.
- El texto de `Insight.Action` proveniente de Groq se muestra tal cual (truncado),
  pero **nunca se interpreta como comando**.

### F5: Refactor — snapshot compartido (sin duplicar lógica)

Extraer de `cmd/doctor.go` a `internal/ai` (nuevo `internal/ai/collect.go`):

- `BuildDoctorSnapshot(analyze func() []core.AnalysisResult) AISnapshot` — hace lo
  que hoy `buildDoctorSnapshot`: resultados + `systemMetrics()` + `core.ReadDiskSnapshot()`
  → `BuildSnapshot`. El parámetro inyectable permite que cada llamador use su
  fuente de análisis (`runAnalysis("")` en CLI, `analysisFn` en TUI, `runAnalysisMCP("")`
  en MCP) y que los tests no toquen el filesystem real.
- `systemMetrics()` y `topCPUProcess()` se mueven tal cual (siguen usando
  `monitor.SystemMonitor`/`ProcessMonitor` y descartan el PID).
- `EngineLabel(engine string) string` se mueve junto (hoy `cmd/doctor.go:engineLabel`)
  para que CLI y TUI muestren el motor idéntico.

Consumidores tras el refactor: `cmd/doctor.go`, el handler MCP `mac_doctor`
(`handleDoctor`) y `internal/tui/doctor.go` llaman a la misma función. Se elimina el
código movido de `cmd/doctor.go` (sin shims). `internal/ai` pasa a importar
`internal/monitor` y `internal/core`; verificar que no haya ciclo (`monitor` y `core`
no deben importar `ai`).

Constraints:
- Comportamiento del CLI (`doctor`, `--json`, `--no-ai`) y de `mac_doctor`
  **idéntico**; solo cambia dónde vive el código.
- Mismo gate de privacidad: el único dato que sale del proceso hacia Groq es el
  `AISnapshot` agregado de `BuildSnapshot` (sin rutas, nombres de archivo/repo ni PIDs).

## Decisiones (cerradas)

1. **Sin ciclo de imports** (verificado): `internal/ai` solo importa `core`; `monitor`
   y `core` no importan `ai`. El armado del snapshot vive en `internal/ai/collect.go`
   (`BuildDoctorSnapshot`, `systemMetrics`, `topCPUProcess`, `EngineLabel`) con la
   función de análisis inyectable. CLI, MCP y TUI usan esa única implementación; la
   privacidad y la salida de `doctor`/`doctor --json` no cambian.
2. **Barra de disco propia** del TUI con lipgloss; no `core.ProgressBar`.
3. **Refresh sin caché:** `d` re-analiza cada vez y `r` refresca en el panel Doctor;
   spinner mientras corre y la llamada a Groq (timeout 20 s) va en `tea.Cmd`.

## Reutilización

| Necesidad | Se reutiliza |
|-----------|--------------|
| Análisis | `analysisFn` (TUI) / `runAnalysis` (CLI) vía parámetro inyectable |
| Métricas de sistema | `monitor.SystemMonitor`, `monitor.ProcessMonitor` (movidas a `ai`) |
| Disco | `core.ReadDiskSnapshot` |
| Snapshot PII-free | `ai.BuildSnapshot` |
| Diagnóstico + fallback | `ai.Diagnose` (heurístico silencioso, timeout 20 s) |
| Etiqueta de motor | `ai.EngineLabel` / `ai.ModelName` |
| Layout | `usableWidth`, `truncateToWidth`, `padLines`, `rule`, `padCell` |
| Paleta | `styles.go` (`riskSafe/Warn/Danger`, `textMuted`, `tableHeaderStyle`) |
| Barra de disco | `proportionBar` (lipgloss, paleta TUI). **No** `core.ProgressBar` |

Decisión: `core.ProgressBar` emite ANSI crudo y rompe el ancho a 80 columnas; el
Doctor (y todo lo nuevo del TUI) usa una barra propia con lipgloss.

## Data model (adiciones)

Un solo campo nuevo en `model`: `doctorOpen bool` (el panel Doctor está en el
viewport; habilita `r`). Doctor es una acción más (`actionItem{label:"Doctor",
group:"Disk", run: runDoctor}`, identificada por la constante `doctorLabel`) cuyo
texto se entrega por `actionResultMsg` al viewport.

```
// internal/tui/doctor.go
var doctorFn = func() ai.Diagnosis  // inyectable en tests (analysisFn + ai.Diagnose)
func runDoctor(width int) (string, error)          // doctorFn → formatDiagnosis
func formatDiagnosis(d ai.Diagnosis, model string, width int) string

// internal/ai/collect.go
func BuildDoctorSnapshot(analyze func() []core.AnalysisResult) AISnapshot
func EngineLabel(engine string) string
func EngineLabelFor(engine, model string) string   // sin depender del entorno
```

`runDoctor` obtiene el modelo con `ai.ModelName()` solo si `d.Engine == ai.EngineGroq`.

## Non-goals (TUI-3)

- Ejecutar acciones desde el Doctor (el puente `c` solo sugiere; no actúa).
- Agente con function-calling → Fase 2 IA.
- `optimize` → Fase 3 IA.
- Marcar items `warn`/`danger` desde el TUI (sigue en TUI-2.1).
- Operaciones Docker desde el TUI.
- Auto-refresh del Doctor, cache de diagnósticos o flag `--no-ai` en el TUI.

## Testing

- **Formateador:** `formatDiagnosis` con un `Diagnosis` fijo (3 secciones + contexto)
  a anchos 40/50/60/80: ninguna línea con `lipgloss.Width > ancho`; secciones vacías
  omitidas; contexto omitido con `DiskTotalBytes == 0`.
- **Motor:** salida contiene `motor: heurístico` para `EngineHeuristic` y
  `motor: groq` (+ modelo si `ModelName` no vacío) para `EngineGroq`.
- **Update:** `tea.KeyMsg{'d'}` en `normalMode` produce `running == true` y un
  `tea.Cmd`; con `doctorFn` inyectado, el `actionResultMsg` resultante llena el
  viewport; `d` mientras `running` no lanza otro cmd; `d` en `selectMode`/`confirmMode`
  no dispara Doctor.
- **Fallback sin key:** con `GROQ_API_KEY` vacío (`t.Setenv`), `ai.Diagnose` devuelve
  `Engine == heuristic` y el panel lo indica; sin error visible.
- **Sin rutas:** con insights/snapshot que contienen rutas simuladas en datos
  de entrada de analyzers, el texto del panel no contiene `/Users/`, `/Library/`
  ni el home del usuario (el Diagnosis heurístico solo trae agregados).
- **Solo lectura:** `cleanerFn` inyectado con contador; ejecutar Doctor y pulsar
  teclas del panel no invoca `Clean()` y `mode` permanece `normalMode`.
- **Layout:** extender `TestViewFitsEightyByTwentyFour` con el panel Doctor cargado
  en el viewport (sin línea > 80 celdas, altura ≤ 24).
- **Refactor:** tests de `ai.BuildDoctorSnapshot` con `analyze` inyectado; los tests
  existentes de `cmd/doctor` y `internal/ai` pasan sin cambios; `go build ./...` y
  `go vet ./...` limpios.
- No se testea el render pixel a pixel ni la llamada real a Groq (ya cubierta por
  `groq_test.go` con servidor httptest).

## Seguridad (resumen — no se relaja nada de SAFETY.md)

- Doctor es **solo lectura**: no borra, no ejecuta comandos, no escribe audit.
- Privacidad: solo el snapshot agregado de `ai.BuildSnapshot` sale hacia Groq, y solo
  si hay `GROQ_API_KEY`; `GROQ_API_KEY` nunca se muestra ni se loguea.
- El TUI no añade rutas de borrado; el puente `c` reutiliza TUI-2 con su gate "si",
  blacklist y audit log intactos.
- Actualizar `docs/SAFETY.md`: añadir una línea al resumen/sección TUI indicando que
  el panel Doctor del TUI es solo lectura (`TUI — Analyze/Status/monitores/Preview/
  Doctor/Last report`). Sin otros cambios.

## Entrega

PR único: `internal/ai/collect.go` (+ tests), `internal/tui/doctor.go` (+ tests),
registro de la acción y atajo `d` en `actions.go`/`model.go` (+ footer), refactor de
`cmd/doctor.go` y del handler MCP `mac_doctor` para usar `ai.BuildDoctorSnapshot`,
extensión de `TestViewFitsEightyByTwentyFour`, `docs/SAFETY.md`, README y CHANGELOG.
Bump esperado: **MINOR** (`v1.7.0`). Flujo: [RELEASE_WORKFLOW.md](../RELEASE_WORKFLOW.md).
