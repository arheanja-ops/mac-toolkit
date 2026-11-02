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
`system_profiler`, `WalkDir`). No crean, modifican ni eliminan archivos.
(`cmd/doctor.go` — "strictly read-only — it never deletes").

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

### (c) La TUI es solo lectura (fase actual)

La TUI (`mac-toolkit tui`) expone únicamente acciones no destructivas: Analyze,
Status, los cuatro monitores, Last report y Quit. No hay ninguna ruta a
`clean`/`--execute` desde la TUI. Un test lo garantiza:
`TestBuildActionsAreReadOnly` falla si se filtra una acción
`clean`/`preview`/`full`. (`internal/tui/actions.go`, `internal/tui/actions_test.go`)

### (d) MCP: tools destructivas con `dry_run=true` por defecto

Las tools de solo lectura (`mac_analyze`, `mac_system`, `mac_battery`, etc.)
nunca borran. Las tools destructivas (`mac_clean_batch`, `mac_docker_cleanup`)
usan `dry_run=true` por defecto: `isDryRun(nil) == true`. Solo un `dry_run=false`
**explícito** ejecuta, y en ese caso el agente debe pedir confirmación al
usuario antes de invocarlas. (`cmd/mcp.go` — `isDryRun`)

### (e) Blacklist de rutas del sistema

Antes de cada borrado, el cleaner consulta la blacklist. Están protegidos de
forma permanente, entre otros: `/System`, `/usr`, `/bin`, `/sbin`,
`/private/var/db` y bundles de Apple. Un intento sobre esas rutas se marca
`skipped-blacklisted`, nunca se elimina. (`internal/cleaner/`)

### (f) Audit log de toda acción destructiva

Cada sesión de limpieza con ejecución real escribe un audit log JSON (con modo
de aprobación y resultado por item) mediante `reporter.AuditReporter`.
(`cmd/clean.go`, `internal/reporter/`)

## Resumen

| Acción | Por defecto | Para ejecutar |
|--------|-------------|---------------|
| `analyze` / `status` / monitores / `doctor` | solo lectura | — (nunca borra) |
| TUI | solo lectura | — (nunca borra) |
| `clean` | dry-run (preview) | `--execute` + aprobación + confirmación `si` |
| MCP read-only tools | solo lectura | — |
| MCP `mac_clean_batch` / `mac_docker_cleanup` | `dry_run=true` | `dry_run=false` explícito + confirmación del usuario |

> Regla de oro: ante cualquier ambigüedad, Mac Toolkit **no borra**.
