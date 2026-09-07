# Guía de Implementación y Ejecución — Mac DevOps Toolkit (CLI Go)

CLI para macOS que cubre limpieza de disco (11 dominios) y monitores de sistema
(batería, CPU, memoria, red). Este documento cubre cómo **compilar**, **ejecutar**,
**verificar** y **extender** la herramienta, además de las notas de fiabilidad de
lecturas del sistema.

- Módulo Go: `mac-toolkit`
- Entrypoint: `main.go` → `cmd.Execute()` (Cobra)
- Binario: `bin/toolkit`

---

## 1. Requisitos

| Requisito | Versión / Nota |
|---|---|
| macOS | Apple Silicon (M1/M2/M3) o Intel |
| Go | 1.22+ (ver `go.mod`) |
| Herramientas del sistema | `ps`, `top`, `vm_stat`, `sysctl`, `pmset`, `ioreg`, `netstat`, `route`, `system_profiler` (todas incluidas en macOS) |
| Docker (opcional) | Solo para el dominio/comandos `docker` |

No requiere permisos de root para monitores ni análisis. La limpieza sólo escribe
si se pasa `--execute` explícitamente.

---

## 2. Compilación (implementación)

Desde la raíz del proyecto (`mac-toolkit/`):

```bash
# Build al binario local bin/toolkit
make build          # equivale a: go build -o bin/toolkit

# Lint (vet) + tests + build
make all            # lint → test → build

# Solo pruebas
make test           # go test ./...

# Solo análisis estático
make lint           # go vet ./...

# Instalar en el PATH del sistema
make install        # go build -o /usr/local/bin/toolkit

# Limpiar artefactos
make clean          # rm -rf bin/
```

Verificación rápida tras compilar:

```bash
./bin/toolkit --help
```

---

## 3. Ejecución

### 3.1 Menú interactivo
Ejecutar sin argumentos abre el menú:

```bash
./bin/toolkit
```

### 3.2 Monitores (solo lectura, no modifican nada)

```bash
./bin/toolkit system      # CPU, memoria, swap, estado térmico, chip/modelo
./bin/toolkit battery     # salud, ciclos, temperatura, voltaje, carga
./bin/toolkit network     # interfaz activa, bytes rx/tx, conexiones, online
./bin/toolkit processes   # top 10 por CPU y por memoria + presión de memoria
```

### 3.3 Análisis de disco (solo lectura)

```bash
# Todos los dominios
./bin/toolkit analyze

# Un dominio concreto
./bin/toolkit analyze --domain docker
./bin/toolkit analyze --domain repos

# Guardar reportes (markdown + JSON)
./bin/toolkit analyze --save

# Umbral mínimo de tamaño (MB) para marcar archivos grandes
./bin/toolkit analyze --min-size 100
```

Dominios disponibles: `xcode`, `ollama`, `trash`, `disk`, `logs`, `docker`,
`dev_caches`, `browser`, `appsupport`, `repos`, `downloads`.

```bash
./bin/toolkit status      # lista dominios y su nivel de riesgo (safe/warn/danger)
```

### 3.4 Limpieza (destructivo solo con --execute)

Por defecto es **dry-run**: no borra nada, solo muestra qué haría.

```bash
# Dry-run (seguro) — muestra preview y no borra
./bin/toolkit clean

# Modos de aprobación
./bin/toolkit clean --mode deal        # (por defecto)
./bin/toolkit clean --mode category
./bin/toolkit clean --mode item
./bin/toolkit clean --mode checklist

# Limpiar un dominio concreto
./bin/toolkit clean --domain dev_caches

# EJECUCIÓN REAL (borra archivos aprobados)
./bin/toolkit clean --execute
```

> Seguridad: `--execute` es la única forma de borrar. Cada ejecución de limpieza
> escribe un log de auditoría vía `AuditReporter`. Rutas del sistema
> (`/System`, `/usr`, `/bin`, `/sbin`, `/private/var/db`) y bundles críticos de
> Apple están en lista negra en `internal/core/config.go` y nunca se tocan.

### 3.5 Docker

```bash
./bin/toolkit docker      # operaciones: backup, cleanup, compact
```

### 3.6 Flujo completo y reportes

```bash
./bin/toolkit full        # analyze + guardar reportes + clean
./bin/toolkit report      # ver reportes guardados
```

### 3.7 Servidor MCP (integración con chats de IA)

```bash
./bin/toolkit mcp         # arranca servidor MCP por stdio
```

Herramientas expuestas: `mac_analyze`, `mac_battery`, `mac_system`,
`mac_processes`, `mac_network`, `mac_status`, `mac_clean_preview`,
`mac_clean_batch`, `mac_docker_backup`, `mac_docker_cleanup`,
`mac_docker_compact`. Ver `docs/MCP_SETUP.md` para la configuración del cliente.

---

## 4. Arquitectura (para extender)

```
main.go                     → cmd.Execute()
cmd/                        → comandos Cobra (analyze, clean, system, battery, ...)
internal/
  core/                     → config, modelos, runner (timeouts), aprobación, logging
  monitor/                  → system, battery, network, processes, exec (helper locale)
  analyzer/                 → un archivo por dominio; registro vía init()+Register()
  cleaner/                  → borrado genérico con dry-run/execute
  reporter/                 → terminal, markdown, json, auditoría
```

### 4.1 Añadir un nuevo dominio de análisis
1. Crear `internal/analyzer/<dominio>.go`.
2. Implementar la interfaz `Analyzer` (`Domain()`, `Risk()`, `Analyze()`).
3. Registrarlo con `func init() { Register(&MiAnalyzer{}) }`.
4. Queda disponible automáticamente en `analyze`, `clean`, `status` y MCP.

### 4.2 Añadir un nuevo monitor
1. Crear `internal/monitor/<monitor>.go` con `Snapshot()` y `Display()`.
2. Añadir el subcomando en `cmd/monitors.go`.
3. **Usar el helper `cmdC(...)` en lugar de `exec.Command(...)`** para cualquier
   comando cuya salida se parsee numéricamente (ver sección 6).

---

## 5. Verificación

```bash
go vet ./...        # análisis estático — debe salir limpio
go test ./...       # tests unitarios (analyzer, cleaner, core)
make build          # compilación

# Verificación funcional de lecturas
./bin/toolkit system && ./bin/toolkit battery && \
  ./bin/toolkit network && ./bin/toolkit processes
```

> Nota macOS: el comando `timeout` no existe por defecto. Usa `gtimeout`
> (`brew install coreutils`) o ejecuta sin envoltura.

---

## 6. Fiabilidad de las lecturas (correcciones aplicadas)

Se corrigieron cuatro fallos que producían lecturas no verídicas. La causa raíz y
el arreglo quedan documentados aquí para evitar regresiones.

### 6.1 %CPU/%MEM de procesos en 0.0 — locale decimal
- **Causa**: `ps` formatea decimales según el locale del usuario. En locales con
  coma decimal (p. ej. `es_CO`), devuelve `87,1` en vez de `87.1`, y
  `strconv.ParseFloat` falla → `0.0`.
- **Arreglo**: helper `cmdC()` en `internal/monitor/exec.go` que fuerza `LC_ALL=C`.
  Aplicado a `ps`, `top`, `vm_stat`, `sysctl`, `memory_pressure`, `pmset`,
  `system_profiler`.
- **Regla**: todo comando del sistema cuyo output se parsee numéricamente debe
  invocarse con `cmdC(...)`, no con `exec.Command(...)`.

### 6.2 Salud de batería reportada como 1%
- **Causa**: en Apple Silicon `MaxCapacity` ya es un **porcentaje** (0–100), no mAh.
  La fórmula previa `MaxCapacity*100/DesignCapacity` daba `100*100/6075 ≈ 1%`.
- **Arreglo** (`internal/monitor/battery.go`):
  - Si `MaxCapacity <= 100` → es el porcentaje de salud directamente.
  - Si no (Intel, mAh) → `AppleRawMaxCapacity / DesignCapacity`.
  - La carga actual usa `CurrentCapacity` como porcentaje en Apple Silicon.

### 6.3 Red mostrando 0.0 MB rx/tx
- **Causa**: la interfaz estaba fijada a `en0`, pero el tráfico real puede ir por
  otra interfaz (p. ej. `en9` con un adaptador/dock USB-Ethernet).
- **Arreglo** (`internal/monitor/network.go`): `defaultInterface()` resuelve la
  interfaz de la ruta por defecto con `route -n get default` y lee sus contadores.
  El display ahora muestra la interfaz activa.

### 6.4 Timeouts en `repos` y `downloads`
- **Causa**: `repos` recorría árboles enormes (incluyendo `.git`) y `downloads`
  descendía sin límite de profundidad; el timeout de 120s no alcanzaba.
- **Arreglo**:
  - `repos.go`: se saltan `.git`, `.hg`, `.svn`, `.Trash`.
  - `downloads.go`: recorrido limitado a 2 niveles de profundidad.
  - `config.go`: `AnalyzerTimeoutSeconds` 120 → 180.

---

## 7. Configuración

Rutas y parámetros viven en `internal/core/config.go`:

- `RepoRoots` — raíces escaneadas por el dominio `repos`.
- `DownloadDirs` — directorios del dominio `downloads`.
- `DevCachePaths`, `BrowserCachePaths`, `LogDirs`, `AppSupportDir`, etc.
- `AnalyzerTimeoutSeconds` — timeout por dominio.
- `DefaultMinSizeMB` — umbral de archivos grandes.
- `BlacklistedPaths` / `BlacklistedPrefixes` — nunca se eliminan.

Ajusta estos valores y recompila (`make build`).

---

## 8. Solución de problemas

| Síntoma | Causa probable | Acción |
|---|---|---|
| Un monitor muestra `0.0` en valores numéricos | Comando nuevo sin `cmdC()` | Usa `cmdC(...)` en el monitor |
| Salud de batería absurda | Regla mAh vs % | Revisar lógica en `battery.go` §6.2 |
| Red en 0.0 MB | Interfaz por defecto no detectada | Verifica `route -n get default` |
| `analyze` da `timeout` en un dominio | Árbol muy grande | Sube `AnalyzerTimeoutSeconds` o acota rutas |
| `timeout: command not found` | macOS no trae `timeout` | `brew install coreutils` → `gtimeout` |

### Nota (deuda técnica detectada, fuera de alcance de esta corrección)
En `cmd/mcp.go`, `handleDockerCleanup` tiene lógica contradictoria al calcular
`dryRun` (varias reasignaciones). No afecta a las lecturas, pero conviene
simplificarlo a una sola asignación en un cambio futuro.
