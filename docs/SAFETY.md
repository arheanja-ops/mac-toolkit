# 🔒 Garantías de seguridad — Mac Toolkit

Mac Toolkit **nunca** borra ni ejecuta acciones destructivas sin autorización
humana explícita. Esta página declara, de forma verificable, qué es seguro por
diseño y qué requiere confirmación. Cada garantía corresponde a código real del
repositorio (archivos citados entre paréntesis).

## Principios

- **Solo lectura por defecto.** Analizar, diagnosticar y monitorear jamás
  modifican el sistema.
- **Dry-run por defecto.** La limpieza no borra nada salvo que el usuario lo
  pida explícitamente.
- **Gate humano inquebrantable.** Toda eliminación exige una confirmación
  interactiva que, ante cualquier duda, falla hacia *no borrar*.
- **Rutas del sistema protegidas.** Una blacklist permanente impide tocar
  directorios críticos de macOS.
- **Auditoría.** Toda acción destructiva queda registrada.

## Garantías concretas

### (a) `doctor`, análisis y monitores son solo lectura

`doctor`, `analyze`, `status`, `battery`, `system`, `processes`, `network` y
`report` únicamente **leen** métricas del sistema (`df`, `ioreg`, `pmset`,
`system_profiler`, `WalkDir`). No crean, modifican ni eliminan archivos
(`analyze --save` solo escribe su propio reporte en `reports/`).
(`cmd/doctor.go` — "strictly read-only — it never deletes").

- **Estados térmicos.** Clasificar `pmset -g therm` como `Normal`, `Throttled`,
  `Warning` o `Unknown` es lectura pura: no cambia ninguna configuración térmica ni
  de energía. Ante salida irreconocible el estado es `Unknown`, nunca una alarma.
  (`internal/monitor/thermal.go` — `parseThermal`, `IsThermalAlert`)
- **`doctor` con IA.** Con `GROQ_API_KEY` el diagnóstico se pide a Groq, pero solo
  se envían **agregados** (tamaños y conteos por dominio, métricas de sistema, nombre
  y % del proceso más caliente): nunca rutas, nombres de archivo ni PIDs; la
  privacidad se impone al construir el snapshot, no confiando en un prompt. El
  resultado son sugerencias en texto: `doctor` no ejecuta ninguna. Si la IA falla,
  cae al motor heurístico local. (`internal/ai/snapshot.go`, `internal/ai/diagnose.go`)

### (b) `clean` es dry-run por defecto + `--execute` + aprobación + confirmación final

El camino por defecto **nunca borra**:

1. Sin `--execute`, `clean` imprime el plan (preview con riesgos) y termina con
   "Dry-run — no files deleted". (`cmd/clean.go`)
2. Con `--execute`, además se exige:
   - **Aprobación interactiva** por modo (`deal`/`category`/`item`/`checklist`)
     vía el `ApprovalEngine`, que ante EOF o entrada vacía responde *no*.
     (`internal/core/approval.go`)
   - **Confirmación final explícita** que indica el número exacto de elementos y
     el tamaño total, y solo procede si el usuario escribe un afirmativo
     inequívoco (`si`/`sí`/`yes`/`y`). Cualquier otra entrada —incluida línea
     vacía o EOF— aborta sin borrar. (`confirmFinalDeletion` en `cmd/clean.go`)

### (c) La TUI puede borrar solo items `safe`, tras selección + confirmación `si`

Desde **TUI-2**, la TUI (`mac-toolkit tui`) ya no es solo lectura: incorpora una
ruta de limpieza interactiva, pero **sin debilitar ninguna garantía**. Reglas:

1. **Solo `safe` es borrable.** Únicamente los items con `Risk == safe &&
   SafeToDelete` son marcables. Los items `warn`/`danger` se muestran atenuados
   con su razón, pero **no se pueden marcar**: el intento es un no-op en el
   modelo (no solo en la UI). (`internal/tui/clean.go` — `toggleMark`,
   `markable`)
2. **Selección explícita + gate `si`.** El usuario marca con checkboxes y pulsa
   `⏎`; aparece un **modal de confirmación** donde debe escribir un afirmativo
   inequívoco (`si`/`sí`/`yes`/`y`) vía la función compartida
   `core.IsAffirmative`. Cualquier otra entrada —incluida vacía o `Esc`— cancela
   sin borrar. Es el mismo gate que el CLI. (`internal/tui/model.go` —
   `onConfirmKey`; `internal/core/affirmative.go`)
3. **Misma ruta destructiva que el CLI.** El borrado usa
   `cleaner.NewGenericCleaner(false, true).Clean()`, que pasa por `cleaner.Delete`
   y por tanto consulta la **blacklist** por cada item. (`internal/tui/clean.go`
   — `runCleanCmd`)
4. **Defensa final.** Antes de borrar, se verifica que **todos** los items
   marcados son `safe`; si se detecta uno no-safe, se **aborta el borrado
   completo** sin eliminar nada. (`internal/tui/clean.go` — `runCleanCmd`,
   `markedItems`)
5. **Audit log siempre.** Toda ejecución real escribe el audit log JSON con
   `reporter.AuditReporter{ApprovalMode: "tui-select"}`, igual que el CLI.
   (`internal/tui/clean.go` — `writeCleanAudit`)
6. **`Preview` nunca borra.** La acción `Preview` de la TUI es un dry-run que
   solo muestra el plan tabulado. (`internal/tui/actions.go` — `runPreview`)

Marcar/borrar items `warn`/`danger` desde la TUI sigue siendo un *non-goal*
(futuro TUI-2.1). En esta fase **solo lo `safe`** es borrable.

**El panel Doctor de la TUI (TUI-3, atajos `d`/`r`) es solo lectura.** No borra,
no ejecuta comandos, no escribe audit y no cambia el modo; su pista "presiona c
para limpiar" solo abre la selección de TUI-2 con su gate `si`, blacklist y audit
log intactos. No muestra rutas, nombres de archivo ni PIDs: solo agregados del
`Diagnosis`. (`internal/tui/doctor.go` — `runDoctor`, `formatDiagnosis`)

### (d) MCP: tools destructivas con `dry_run=true` por defecto

El servidor expone 12 tools. Las 9 de solo lectura (`mac_analyze`, `mac_battery`,
`mac_system`, `mac_processes`, `mac_network`, `mac_status`, `mac_clean_preview`,
`mac_docker_compact`, `mac_doctor`) nunca borran. Las 2 destructivas
(`mac_clean_batch`, `mac_docker_cleanup`) usan `dry_run=true` por defecto:
`isDryRun(nil) == true`. Solo un `dry_run=false` **explícito** ejecuta, y en ese caso
el agente debe pedir confirmación al usuario antes de invocarlas. (`cmd/mcp.go` —
`isDryRun`)

Matices, verificados en el código:

- `mac_clean_batch` solo borra items `safe_to_delete`, a través del cleaner (la
  blacklist aplica), pero **no escribe audit log**.
- `mac_docker_backup` no borra: escribe un dump en `~/docker-backups/`. No tiene
  parámetro `dry_run`.

### (e) Blacklist de rutas del sistema

Antes de cada borrado, el cleaner consulta la blacklist. Están protegidos de
forma permanente, entre otros: `/System`, `/usr`, `/bin`, `/sbin`,
`/private/var/db` y bundles de Apple. Un intento sobre esas rutas se marca
`skipped-blacklisted`, nunca se elimina. (`internal/cleaner/`)

### (f) Audit log de la limpieza de archivos (CLI y TUI)

Cada sesión de `clean --execute` (CLI) y cada borrado desde la TUI escribe un audit
log JSON (`reports/cleanup_<fecha_hora>/audit.json`, con modo de aprobación y
resultado por item) mediante `reporter.AuditReporter`. (`cmd/clean.go`,
`internal/tui/clean.go`, `internal/reporter/audit.go`)

### (g) Docker: límites conocidos

`mac-toolkit docker cleanup` es dry-run por defecto y solo borra con `--execute`
(requiere `--keep`); la tool MCP `mac_docker_cleanup` es `dry_run=true` por
defecto. Ambos llaman a `internal/docker` directamente: **no** pasan por el gate
`si`, la blacklist del cleaner ni el audit log. La protección es el dry-run por
defecto y la lista `keep`; revisa siempre el dry-run antes de ejecutar.
(`cmd/docker.go`, `internal/docker/ops.go`)

## Resumen

| Acción | Por defecto | Para ejecutar |
|--------|-------------|---------------|
| `analyze` / `status` / monitores / `doctor` | solo lectura | — (nunca borra) |
| TUI — Analyze/Status/monitores/Preview/Last report | solo lectura | — (nunca borra) |
| TUI — Clean (solo items `safe`) | preview/selección | selección explícita + confirmación `si` (blacklist + audit) |
| `clean` | dry-run (preview) | `--execute` + aprobación + confirmación `si` |
| Estados térmicos (`system`, header, `mac_system`) | solo lectura | — (nunca modifica nada) |
| `docker backup` / MCP `mac_docker_backup` | escribe un dump en `~/docker-backups/` | — (no borra; sin `dry_run`) |
| `docker cleanup` | dry-run | `--keep` + `--execute` (sin gate `si`, blacklist ni audit) |
| MCP read-only tools (9, incl. `mac_doctor`) | solo lectura | — |
| MCP `mac_clean_batch` / `mac_docker_cleanup` | `dry_run=true` | `dry_run=false` explícito + confirmación del usuario |

> Regla de oro: ante cualquier ambigüedad, Mac Toolkit **no borra**.
