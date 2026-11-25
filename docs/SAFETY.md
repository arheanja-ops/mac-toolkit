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

`doctor`, `analyze`, `status`, `battery`, `system`, `memory`, `cpu`, `processes`, `network` y
`report` únicamente **leen** métricas del sistema (`df`, `ioreg`, `pmset`,
`system_profiler`, `WalkDir`). No crean, modifican ni eliminan archivos
(`analyze --save` y `Save report` de la TUI solo escriben su propio reporte en la carpeta de reportes del toolkit, `os.UserConfigDir()/mac-toolkit/reports`).
(`cmd/doctor.go` — "strictly read-only — it never deletes").

- **Estados térmicos.** Clasificar `pmset -g therm` como `Normal`, `Throttled`,
  `Warning` o `Unknown` es lectura pura: no cambia ninguna configuración térmica ni
  de energía. Ante salida irreconocible el estado es `Unknown`, nunca una alarma.
  (`internal/monitor/thermal.go` — `parseThermal`, `IsThermalAlert`)
- **Vistas Memory y CPU (v1.8.0).** `memory`, `cpu` y las acciones `Memory`/`CPU` de la
  TUI (`m`/`u`/`r`) solo ejecutan `vm_stat`, `sysctl`, `memory_pressure`, `top`, `ps` y
  `pmset` (sin shell) y formatean el resultado. No envían señales ni matan procesos, no
  escriben nada y no tienen teclas que muten. Agrupan por aplicación y no muestran PIDs
  ni rutas. (`internal/monitor/memory.go`, `internal/monitor/cpu.go`, `internal/tui/memory_view.go`)
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

### (d) MCP: tools destructivas con preview, opt-in del humano y `confirm_plan`

El servidor expone 12 tools. Las 9 de solo lectura (`mac_analyze`, `mac_battery`,
`mac_system`, `mac_processes`, `mac_network`, `mac_status`, `mac_clean_preview`,
`mac_docker_compact`, `mac_doctor`) nunca borran. Las 2 destructivas
(`mac_clean_batch`, `mac_docker_cleanup`) tienen **tres** barreras técnicas
(`cmd/mcp_destructive.go`):

1. **Preview por defecto.** `dry_run=true` si se omite (`isDryRun(nil) == true`).
   La preview devuelve el plan y un `plan_digest` (SHA-256 de los identificadores
   ordenados del plan; en `mac_clean_batch`, de las rutas `safe_to_delete`
   seleccionadas).
2. **Opt-in del humano.** Con `dry_run=false` el servidor **rechaza** salvo que su
   proceso tenga `MAC_TOOLKIT_MCP_ALLOW_DELETE=1`. La variable la fija el humano en
   el bloque `env` de su configuración MCP; el agente no puede concedérsela. El
   rechazo explica el opt-in y el comando de terminal equivalente
   (`mac-toolkit docker cleanup --keep X --execute` / `mac-toolkit clean --execute`).
3. **`confirm_plan`.** Con `dry_run=false` se exige `confirm_plan` igual al
   `plan_digest` del plan **recalculado en ese momento**. Si falta o difiere (el
   estado cambió desde la preview) se rechaza sin borrar y se pide una nueva preview.

Tras ejecutar se escribe el **audit log** (`ApprovalMode: "mcp"`) y se devuelven los
resultados por item, **incluidos los fallos**. `mac_clean_batch` sigue pasando por
`cleaner.Delete` (blacklist) y solo toca items `safe_to_delete`.

Matiz: `mac_docker_backup` no borra: escribe un dump en `~/docker-backups/`. No
tiene parámetro `dry_run`.

### (e) Blacklist de rutas del sistema

Antes de cada borrado, el cleaner consulta la blacklist. Están protegidos de
forma permanente, entre otros: `/System`, `/usr`, `/bin`, `/sbin`,
`/private/var/db` y bundles de Apple. Un intento sobre esas rutas se marca
`skipped-blacklisted`, nunca se elimina. (`internal/cleaner/`)

### (f) Audit log de toda eliminación (CLI, TUI, Docker y MCP)

Cada sesión de `clean --execute` (CLI), cada borrado desde la TUI, cada
`docker cleanup --execute` (`docker-cli`) y cada ejecución real de las tools MCP
destructivas (`mcp`) escribe un audit log JSON
(`<carpeta de reportes>/cleanup_<fecha_hora>/audit.json`, donde la carpeta es `os.UserConfigDir()/mac-toolkit/reports`, p. ej. `~/Library/Application Support/mac-toolkit/reports` en macOS, con modo de aprobación y resultado por
item) mediante `reporter.AuditReporter`. (`cmd/clean.go`, `cmd/docker_cleanup.go`,
`cmd/mcp_destructive.go`, `internal/tui/clean.go`, `internal/reporter/audit.go`)

### (g) Docker: plan, gate `si` y audit

`internal/docker` separa **planificar** de **aplicar**:

- `Plan(keep)` es de **solo lectura**: lista contenedores, imágenes y volúmenes a
  eliminar, lo que se conserva (contenedores `keep` + sus imágenes y volúmenes) y el
  build cache reclamable. Rechaza un `keep` vacío y también un `keep` que no existe
  (un typo dejaría sus volúmenes sin proteger).
- `Apply(plan, runner)` ejecuta **únicamente** lo que está en el plan y devuelve un
  resultado por item (`success`/`failure`/`skipped`); los errores **no se ignoran**.
  Nunca ejecuta items con nombre vacío o que parezca un flag, de tipo desconocido, ni
  un contenedor que el propio plan marca como conservado.
- El `Runner` (exec de docker) es inyectable: los tests no tocan Docker real.

`mac-toolkit docker cleanup --keep X`:

1. Sin `--execute` es **dry-run**: muestra el plan y no borra.
2. Con `--execute` muestra el plan completo, resalta los volúmenes en una línea
   aparte (`⚠ Volúmenes: pueden contener datos (bases de datos)`), indica el conteo
   exacto y exige un afirmativo (`core.IsAffirmative`). Vacío, EOF o cualquier otra
   cosa → `Aborted — nothing was deleted.` sin invocar docker.
3. Tras aplicar escribe el audit log (`ApprovalMode: "docker-cli"`; rutas
   `docker:container:<n>`, `docker:image:<id|ref>`, `docker:volume:<n>`,
   `docker:build-cache`) y muestra éxitos y fallos.

Límite que se mantiene: la blacklist de rutas (e) protege rutas de archivo y no
aplica a recursos Docker; su protección es la lista `keep`, el plan revisado y el gate
humano. Docker rechaza por sí mismo `rmi`/`volume rm` de recursos en uso por los
contenedores conservados, y esos rechazos se informan como fallos.
(`internal/docker/plan.go`, `cmd/docker_cleanup.go`)

## Resumen

| Acción | Por defecto | Para ejecutar |
|--------|-------------|---------------|
| `analyze` / `status` / monitores / `doctor` | solo lectura | — (nunca borra) |
| TUI — Analyze/Status/monitores (incl. vistas Memory y CPU)/Preview/Doctor/Last report | solo lectura | — (nunca borra ni envía señales a procesos) |
| TUI — Clean (solo items `safe`) | preview/selección | selección explícita + confirmación `si` (blacklist + audit) |
| `clean` | dry-run (preview) | `--execute` + aprobación + confirmación `si` |
| Estados térmicos (`system`, header, `mac_system`) | solo lectura | — (nunca modifica nada) |
| `docker backup` / MCP `mac_docker_backup` | escribe un dump en `~/docker-backups/` | — (no borra; sin `dry_run`) |
| `docker cleanup` | dry-run (muestra el plan) | `--keep` + `--execute` + confirmación `si` (+ audit `docker-cli`) |
| MCP read-only tools (9, incl. `mac_doctor`) | solo lectura | — |
| MCP `mac_clean_batch` / `mac_docker_cleanup` | `dry_run=true` (preview + `plan_digest`) | `dry_run=false` + `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` en el servidor + `confirm_plan` = `plan_digest` vigente (+ audit `mcp`) |

> Regla de oro: ante cualquier ambigüedad, Mac Toolkit **no borra**.
