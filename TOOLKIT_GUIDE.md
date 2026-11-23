# 📖 Mac Toolkit — Guía Completa

Guía de referencia para el Mac Toolkit: CLI nativa en Go para supervisión, limpieza y optimización de macOS, con integración MCP para usar desde chat.

---

## Tabla de Contenidos

1. [Instalación](#instalación)
2. [Uso por CLI](#uso-por-cli)
   - [TUI interactiva](#tui-interactiva-estilo-k9s) · [`doctor`](#diagnóstico-doctor)
3. [MCP Server — Uso desde Chat](#mcp-server--uso-desde-chat)
4. [Configuración en Kiro Desktop (IDE)](#configuración-en-kiro-desktop-ide)
5. [Configuración en Kiro CLI](#configuración-en-kiro-cli)
6. [Herramientas MCP disponibles](#herramientas-mcp-disponibles)
7. [Dominios de análisis](#dominios-de-análisis)
8. [Monitors del sistema](#monitors-del-sistema)
9. [Modos de aprobación para limpieza](#modos-de-aprobación-para-limpieza)
10. [Seguridad](#seguridad)
11. [Agentes Kiro](#agentes-kiro)
12. [Skills Kiro](#skills-kiro)
13. [Desarrollo y extensibilidad](#desarrollo-y-extensibilidad)
14. [Troubleshooting](#troubleshooting)
15. [Flujo de contribución y release](#flujo-de-contribución-y-release)

---

## Instalación

### Recomendado: `go install`

```bash
go install github.com/arheanja-ops/mac-toolkit@latest
```

Esto instala el binario como `mac-toolkit` en `$(go env GOPATH)/bin`. Asegúrate de que ese directorio esté en tu `PATH`.

### Desde código fuente

```bash
git clone https://github.com/arheanja-ops/mac-toolkit.git
cd mac-toolkit
make build     # genera ./bin/toolkit
```

### Verificar

```bash
mac-toolkit --help
mac-toolkit --version  # con go install @vX.Y.Z muestra la versión (commit/fecha: none/unknown)
mac-toolkit status     # lista dominios registrados
```

`--version` (`cmd/root.go`): los binarios del release traen versión, commit y fecha
inyectados por GoReleaser; con `go install ...@vX.Y.Z` la versión se lee de
`runtime/debug.ReadBuildInfo()` y commit/fecha quedan como `none`/`unknown`.

---

## Uso por CLI

### TUI interactiva (estilo k9s)

```bash
mac-toolkit          # sin argumentos abre la TUI de dos paneles
mac-toolkit tui      # equivalente explícito
```

Tras una pantalla de bienvenida (banner + resumen del sistema), la TUI muestra:

- **Header en vivo** (refresco cada 2 s): disco, memoria, CPU y estado térmico.
- **Panel izquierdo** — acciones: Disk (Analyze, Status, Doctor), Clean (Preview,
  Select & clean), Monitors (Battery, System, Memory, CPU, Processes, Network), Reports (Last
  report) y Session (Quit / Salir).
- **Panel derecho** — resultado de la acción, con scroll. Responsive hasta 80x24.

| Modo | Teclas |
|------|--------|
| Normal | `↑↓` mover · `Tab` cambiar panel · `⏎` ejecutar · `/` filtrar · `a` Analyze · `p` Preview · `c` modo selección de limpieza · `d` Doctor · `m` Memory · `u` CPU · `r` refrescar la vista abierta (Doctor/Memory/CPU) · `q`/`Ctrl+C`/`Esc` salir |
| Selección de limpieza | `↑↓` (o `k`/`j`) mover · `espacio` marcar · `a` todos los `safe` · `n` ninguno · `⏎` continuar · `Esc` cancelar |
| Confirmación | escribir `si` + `⏎` borra; `Esc` u otra entrada cancela |

**Limpieza en la TUI.** Solo los items `safe` (`Risk == safe` y `SafeToDelete`) son
marcables; `warn`/`danger` se muestran atenuados con su razón y no se pueden marcar.
Al continuar aparece el modal "si": sin un afirmativo inequívoco no se borra nada.
El borrado usa el mismo cleaner (con blacklist) y escribe el mismo audit log que el
CLI (`approval_mode: tui-select`). `Preview` es dry-run y nunca borra. Detalle en
[`docs/SAFETY.md`](docs/SAFETY.md).

**Panel Doctor.** `d` (o la acción Doctor) ejecuta el diagnóstico y lo muestra en
tres grupos: PERFORMANCE, QUICK WINS y REQUIERE REVISIÓN, más el contexto de disco y
el motor usado. Solo lectura; cada pulsación re-analiza sin caché.

### Menú lineal (fallback)

```bash
mac-toolkit menu     # menú con flechas (promptui): Disk, Monitors, Reports
```

Incluye la opción `Cleanup preview (dry-run, shows risks)` para revisar qué se puede limpiar sin borrar nada.

### Diagnóstico: `doctor`

```bash
mac-toolkit doctor            # plan priorizado (performance / quick wins / revisión)
mac-toolkit doctor --no-ai    # fuerza el motor heurístico determinista
mac-toolkit doctor --json     # Diagnosis como JSON por stdout, para scripting
```

`doctor` ejecuta todos los analyzers y monitores, arma un agregado **sin PII** y lo
convierte en un diagnóstico priorizado. Es **estrictamente solo lectura**: nunca
borra ni ejecuta acciones.

- **Motor:** usa Groq si `GROQ_API_KEY` está definida y no se pasó `--no-ai`; si no
  hay clave, o la llamada falla o excede 20 s, cae en silencio al motor heurístico.
- **Variables:** `GROQ_API_KEY` (sin default → heurístico), `GROQ_MODEL` (default
  `openai/gpt-oss-20b`), `GROQ_BASE_URL` (default
  `https://api.groq.com/openai/v1`, endpoint OpenAI-compatible).
- **Privacidad:** al usar IA solo se envían agregados (tamaños por dominio,
  severidad, conteos, métricas de CPU/memoria/swap/térmico y el nombre y % del
  proceso más caliente). Nunca rutas, nombres de archivo, nombres de repos ni PIDs
  (`internal/ai/snapshot.go`). El dominio `disk` es solo contexto y no cuenta como
  espacio recuperable.
- **Disponible en:** CLI, TUI (`d`) y la tool MCP `mac_doctor`.

### Comandos de análisis

```bash
mac-toolkit analyze                          # Análisis completo (11 dominios en paralelo)
mac-toolkit analyze --domain browser         # Solo caches de navegador
mac-toolkit analyze --domain docker          # Solo Docker
mac-toolkit analyze --save                   # Guardar reporte MD + JSON en reports/
mac-toolkit analyze --min-size 100           # Umbral de archivos grandes (MB, default 50)
```

### Comandos de limpieza

`mac-toolkit clean` **siempre** muestra primero el plan completo en dry-run: una tabla por dominio con riesgo, tamaño, edad, si es seguro (safe) y ruta, más los totales y una leyenda de riesgos. No borra nada. Sin `--execute` se detiene ahí.

Con `--execute` continúa a la aprobación interactiva y muestra una tabla final antes de borrar.

```bash
mac-toolkit clean                            # Plan dry-run (no borra nada)
mac-toolkit clean --execute                  # Aprobación interactiva + borrado
mac-toolkit clean --execute --mode checklist # Selección manual por checklist
mac-toolkit clean --execute --domain logs    # Solo logs
mac-toolkit full --execute                   # Analizar + guardar + limpiar
```

Con `--execute`, tras la aprobación por modo se exige una **confirmación final**:
el CLI muestra el número de elementos y el tamaño total y hay que escribir `si`
(`si`/`sí`/`yes`/`y`). Cualquier otra entrada, incluida una línea vacía o EOF, aborta
sin borrar.

### Docker

```bash
mac-toolkit docker backup <contenedor>       # Dump de BD (postgres/mysql) a ~/docker-backups/
mac-toolkit docker cleanup --keep a,b        # Dry-run: elimina todo salvo esos contenedores
mac-toolkit docker cleanup --keep a,b --execute   # Borra de verdad
mac-toolkit docker compact                   # Docker.raw: tamaño virtual vs uso real
```

> `docker cleanup --execute` no pasa por el gate `si` del `clean` ni por la
> blacklist/audit del cleaner: la única protección es el dry-run por defecto y
> `--keep`. Revisa siempre el dry-run primero.

### Monitors

```bash
mac-toolkit battery      # Salud, ciclos, temperatura, voltaje
mac-toolkit system       # CPU, memoria, swap, estado térmico (Normal/Throttled/Warning/Unknown), modelo
mac-toolkit memory       # RAM usada (def. Monitor de Actividad), presión, swap, top 5 apps
mac-toolkit cpu          # user/sys/idle (2.ª muestra de top), load por núcleo, núcleos P/E, top 5 apps
mac-toolkit processes    # Top 10 por CPU + top 10 por memoria
mac-toolkit network      # WiFi, tráfico, conexiones, conectividad
```

### Otros

```bash
mac-toolkit status           # Dominios y niveles de riesgo
mac-toolkit report --last    # Ver último reporte guardado
mac-toolkit mcp              # Iniciar MCP server (stdio)
mac-toolkit completion zsh   # Autocompletado de shell (bash|zsh|fish|powershell)
```

---

## MCP Server — Uso desde Chat

El toolkit incluye un MCP server que expone todas sus capacidades como herramientas invocables desde cualquier cliente MCP: Kiro Desktop, Kiro CLI, Claude Desktop, etc.

### ¿Qué es MCP?

MCP (Model Context Protocol) es un protocolo estándar que permite a herramientas de IA invocar funcionalidades externas. El toolkit se ejecuta como un proceso hijo que se comunica por stdin/stdout (JSON-RPC).

### Iniciar manualmente

```bash
mac-toolkit mcp    # inicia el server en stdio
```

No necesitas iniciarlo manualmente — se configura en el agente y Kiro lo inicia automáticamente.

---

## Configuración en Kiro Desktop (IDE)

Kiro Desktop lee los mismos archivos `.kiro/` del workspace. Hay tres formas de conectar el toolkit:

### Opción 1: MCP en workspace (recomendada)

Crear `.kiro/settings/mcp.json` en el directorio del proyecto:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

Esto hace que las 12 herramientas estén disponibles en **cualquier agente** cuando trabajas en este workspace.

### Opción 2: MCP global (disponible en todos los proyectos)

Editar `~/.kiro/settings/mcp.json`:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

### Opción 3: Usar el agente mac-ops

Los agentes en `.kiro/agents/` son detectados automáticamente por Kiro Desktop. Abre el proyecto `mac-toolkit/` en Kiro Desktop y el agente `mac-ops` aparecerá disponible para seleccionar.

### Verificar en Kiro Desktop

1. Abre Kiro Desktop en el directorio `mac-toolkit/`
2. En el chat, escribe `/mcp` — verás `mac-toolkit` con status ✓
3. Las herramientas `mac_analyze`, `mac_battery`, etc. estarán disponibles
4. Pregunta: "¿cómo está mi mac?" y el agente usará las herramientas MCP

### Verificar con Kiro CLI

```bash
# Desde el directorio mac-toolkit/
kiro-cli mcp list                    # ver servers configurados
kiro-cli mcp status --name mac-toolkit  # ver estado del server
```

---

## Configuración en Kiro CLI

### Opción 1: Usar el agente mac-ops directamente

```bash
cd mac-toolkit
kiro-cli chat --agent mac-ops
```

Esto inicia una sesión con el MCP server ya configurado. Puedes preguntar en español:

```
> ¿cómo está mi mac?
> analiza el disco
> qué puedo limpiar
> revisa la batería
> cómo está la red
```

### Opción 2: Agregar MCP via CLI

```bash
kiro-cli mcp add \
  --name mac-toolkit \
  --command mac-toolkit \
  --args mcp
```

### Opción 3: Shortcut de teclado

Con el agente `mac-ops` configurado, puedes cambiar a él con `Ctrl+Shift+M` durante cualquier sesión de chat.

### Opción 4: Agente por defecto

```bash
kiro-cli settings chat.defaultAgent mac-ops
kiro-cli chat    # arranca directamente con mac-ops
```

---

## Herramientas MCP disponibles

El toolkit expone **12 herramientas** MCP (fuente: `mcp.AddTool` en `cmd/mcp.go`):
9 de solo lectura, 2 destructivas con `dry_run=true` por defecto y 1 que escribe un
archivo sin borrar nada.

### Solo lectura (9)

| Herramienta | Descripción | Parámetros |
|-------------|-------------|------------|
| `mac_analyze` | Análisis de disco completo o por dominio. Retorna JSON con severidad, tamaño, items, y riesgo por dominio. | `domain` (opcional): browser, docker, ollama, logs, dev_caches, xcode, repos, downloads, appsupport, trash, disk |
| `mac_battery` | Batería: salud %, ciclos, temperatura °C, voltaje V, carga actual, tiempo restante, estado de carga. | — |
| `mac_system` | Sistema: modelo, chip, cores, RAM, CPU %, memoria usada/total, swap, estado térmico (Normal/Throttled/Warning/Unknown). | — |
| `mac_processes` | Top 10 procesos por CPU y top 10 por memoria, con PID, RSS, y memory pressure. | — |
| `mac_network` | Red: WiFi SSID/señal/canal, bytes enviados/recibidos, conexiones activas, check de conectividad. | — |
| `mac_status` | Lista todos los dominios registrados con su nivel de riesgo (safe/warn/danger). | — |
| `mac_clean_preview` | Preview de limpieza: items seguros vs inseguros, tamaños, razones. Nunca borra nada. | `domain` (opcional) |
| `mac_docker_compact` | Analiza `Docker.raw` (tamaño virtual vs uso real) y da instrucciones para recuperar espacio. No modifica nada. | — |
| `mac_doctor` | Diagnóstico priorizado (performance, quick_wins, review) + contexto de disco y bytes recuperables seguros. Groq si hay `GROQ_API_KEY`; si no, heurístico. Sin PII. | `no_ai` (opcional): fuerza el motor heurístico |

### Destructivas — `dry_run=true` por defecto (2)

Solo un `dry_run=false` **explícito** puede ejecutar borrados (`isDryRun(nil) == true`), y además requiere `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` en el servidor y `confirm_plan` = `plan_digest` de la preview (ver `docs/SAFETY.md`);
el agente debe pedir confirmación al usuario antes.

| Herramienta | Descripción | Parámetros |
|-------------|-------------|------------|
| `mac_clean_batch` | Limpieza por lotes de items `safe_to_delete`. Con `dry_run=true` (default) solo muestra el plan; con `dry_run=false` borra. Dominios válidos: browser, dev_caches, logs, trash, xcode, ollama, downloads, appsupport. | `domains` (lista, opcional; vacío = todos), `dry_run` (default true), `confirm_plan` (requerido con `dry_run=false`) |
| `mac_docker_cleanup` | Elimina recursos Docker (contenedores, imágenes, volúmenes) excepto los de `keep`. | `keep` (requerido, nombres separados por coma), `dry_run` (default true), `confirm_plan` (requerido con `dry_run=false`) |

### Escribe un archivo, no borra (1)

| Herramienta | Descripción | Parámetros |
|-------------|-------------|------------|
| `mac_docker_backup` | Dump de la BD de un contenedor **en ejecución** (detecta postgres/mysql por imagen; `pg_dumpall`/`mysqldump`) y lo verifica. Escribe en `~/docker-backups/`. No tiene `dry_run`. | `container` (requerido) |

### Ejemplo de respuesta de `mac_analyze`

```json
{
  "results": [
    {
      "domain": "browser",
      "severity": "medium",
      "total_size": "450.2 MB",
      "item_count": 3,
      "summary": "Browser caches: 450.2 MB across 3 browsers",
      "items": [
        {
          "path": "/Users/<usuario>/Library/Caches/Google/Chrome",
          "size": "280.5 MB",
          "label": "Chrome cache",
          "safe_to_delete": true,
          "risk": "safe",
          "reason": "Browser cache — auto-regenerated"
        }
      ]
    }
  ],
  "summary": "Analyzed 11 domains: 15.2 GB total across 47 items"
}
```

---

## Dominios de análisis

| Dominio | Qué analiza | Riesgo | Auto-regenera |
|---------|-------------|--------|---------------|
| `disk` | Uso del volumen APFS `/System/Volumes/Data` | — | N/A |
| `ollama` | Modelos LLM en `~/.ollama/models/blobs/` | 🔴 danger | No (re-download) |
| `docker` | Imagen virtual `Docker.raw` | 🔴 danger | No (destruye todo) |
| `browser` | Caches de Chrome, Safari, Firefox, Edge | 🟢 safe | Sí |
| `logs` | Archivos `.log*` > 7 días en Library/Logs, /var/log | 🟢 safe | Sí |
| `dev_caches` | npm `_cacache`, pip cache, Homebrew cache | 🟢 safe | Sí |
| `repos` | `node_modules`, `.venv`, `__pycache__`, `.pytest_cache` | 🟢 safe | Sí |
| `xcode` | DerivedData (safe), Simulators (safe), Archives (warn) | 🟢/🟡 | Parcial |
| `downloads` | Archivos grandes >50MB, duplicados, archivos (.zip/.dmg/.pkg) | 🟡 warn | No |
| `appsupport` | `~/Library/Application Support` dirs >50MB | 🟡 warn | No |
| `trash` | `~/.Trash` | 🟡 warn | No |

---

## Monitors del sistema

### Battery
- **Fuente**: `ioreg -rn AppleSmartBattery` + `pmset -g batt`
- **Métricas**: health %, cycle count, temperatura °C, voltaje V, carga actual %, tiempo restante, estado de carga
- **Colores**: 🟢 ≥80% salud, 🟡 ≥60%, 🔴 <60%

### System
- **Fuente**: `system_profiler SPHardwareDataType -json`, `top`, `vm_stat`, `sysctl`, `pmset -g therm`
- **Métricas**: modelo, chip, cores, RAM, CPU %, memoria usada/total/%, swap, estado térmico
- **Estado térmico** (`internal/monitor/thermal.go`): `Normal` (sin avisos o `CPU_Speed_Limit` = 100), `Throttled` (`CPU_Speed_Limit` < 100), `Warning` (nivel de aviso realmente registrado) o `Unknown` (salida vacía/irreconocible o `pmset` falló). Solo `Throttled` y `Warning` son alerta; es lectura pura
- **Colores CPU**: 🟢 <60%, 🟡 <85%, 🔴 ≥85%
- **Colores Memoria**: 🟢 <70%, 🟡 <90%, 🔴 ≥90%

### Processes
- **Fuente**: `ps -arcwwwxo pid,pcpu,pmem,rss,comm`, `memory_pressure`
- **Métricas**: top 10 CPU, top 10 memoria (PID, CPU%, MEM%, RSS, nombre)

### Network
- **Fuente**: `netstat -ib`, airport framework, `netstat -an`, HTTP check a apple.com
- **Métricas**: WiFi SSID/RSSI/canal, bytes enviados/recibidos, conexiones activas/total, online/offline

---

## Modos de aprobación para limpieza

| Modo | Flag | Comportamiento |
|------|------|---------------|
| **deal** | `--mode deal` (default) | Muestra total reclamable → pregunta por categoría |
| **category** | `--mode category` | Pregunta s/N por cada dominio |
| **item** | `--mode item` | Pregunta s/N por cada archivo individual |
| **checklist** | `--mode checklist` | Lista numerada, selecciona con números o "all"/"todos" |

Acepta español e inglés: `s`, `si`, `sí`, `y`, `yes`

---

## Seguridad

### Dry-run por defecto
`mac-toolkit clean` siempre muestra primero el plan completo en dry-run y **nunca borra nada** sin `--execute`. Con `--execute` pasa a aprobación interactiva y una tabla final antes de borrar.

### Blacklist permanente
Estos paths están protegidos y nunca se borran:
- `/System`, `/usr`, `/bin`, `/sbin`, `/private/var/db`
- `com.apple.dock`, `com.apple.finder`, `com.apple.spotlight`

### Confirmación final "si"
**Memory y CPU (solo lectura).** `m` y `u` abren vistas dedicadas; `r` las refresca
(solo manual, con spinner; CPU tarda ~2 s). La *memoria usada* sigue la definición de
Monitor de Actividad: `App Memory (Anonymous − Purgeable) + Wired + Compressed`; es la
misma cifra en `system`, el header, Memory y Doctor. Las apps se agrupan por aplicación
y nunca se muestran PIDs ni rutas. El Doctor añade insights de CPU saturada, tiempo de
kernel elevado y paginación intensa (solo con números agregados).

Con `--execute`, `clean` exige además escribir `si` tras ver el número exacto de elementos y el tamaño total. La TUI usa el mismo gate (`core.IsAffirmative`) en su modal.

### TUI
Solo borra items `safe`, tras selección explícita y confirmación `si`; pasa por la blacklist y escribe audit log. `Preview` y el panel Doctor nunca borran.

### `doctor` y monitores
Solo lectura (CLI, TUI y `mac_doctor`). Con IA solo se envían agregados, nunca rutas ni PIDs.

### Audit log
Cada sesión de limpieza real (CLI y TUI) genera un `audit.json` en `reports/cleanup_<fecha_hora>/` con:
- UUID de sesión
- Usuario que ejecutó
- Flag dry-run
- Modo de aprobación
- Lista de eliminaciones con resultado (success/failure/skipped)

### MCP: seguro por defecto
De las 12 herramientas MCP, 9 son de solo lectura (análisis, monitoreo, preview, doctor) y nunca modifican el disco. Las 2 destructivas (`mac_clean_batch`, `mac_docker_cleanup`) usan `dry_run=true` por defecto: solo muestran el plan (con `plan_digest`); para ejecutar cambios reales requieren `dry_run=false`, el opt-in `MAC_TOOLKIT_MCP_ALLOW_DELETE=1` en el servidor y `confirm_plan` igual al `plan_digest`. `mac_docker_backup` escribe un dump en `~/docker-backups/` y no borra nada.

> Matices verificados en el código: `mac_clean_batch` borra a través del cleaner (blacklist activa); ambas tools destructivas escriben audit log (`mcp`). `docker cleanup` (CLI) muestra el plan, exige el gate `si` y escribe audit log (`docker-cli`); la blacklist de rutas no aplica a recursos Docker. Ver [`docs/SAFETY.md`](docs/SAFETY.md).

---

## Agentes Kiro

### mac-ops (Ctrl+Shift+M)
Agente para **operaciones** — supervisión y análisis del Mac desde chat.

```bash
kiro-cli chat --agent mac-ops
```

Ejemplos de conversación:
- "¿cómo está mi mac?" → sistema + batería + procesos
- "analiza el disco" → análisis completo
- "qué puedo limpiar" → preview de limpieza
- "revisa la red" → estado de red

### mac-toolkit-dev (Ctrl+Shift+T)
Agente para **desarrollo** — extender y modificar el toolkit.

```bash
kiro-cli chat --agent mac-toolkit-dev
```

Tiene acceso a las herramientas MCP + herramientas de desarrollo (read, write, shell, code).

---

## Skills Kiro

Skills invocables como `/skill-name` en el chat:

| Skill | Uso | Descripción |
|-------|-----|-------------|
| `/analyze-domain` | `/analyze-domain browser` | Ejecutar análisis y explicar resultados |
| `/add-analyzer` | `/add-analyzer homebrew` | Scaffolding de nuevo analyzer |
| `/add-monitor` | `/add-monitor gpu` | Scaffolding de nuevo monitor |
| `/toolkit-test` | `/toolkit-test core` | Ejecutar tests y analizar fallos |
| `/toolkit-build` | `/toolkit-build release` | Build completo con verificación |
| `/mac-optimize` | `/mac-optimize clean` | Análisis completo + recomendaciones |

---

## Desarrollo y extensibilidad

### Agregar un nuevo analyzer

1. Crear `internal/analyzer/mianalyzer.go`
2. Implementar la interfaz `Analyzer` (Domain, Risk, Analyze)
3. Llamar `Register(&MiAnalyzer{})` en `init()`
4. Se auto-registra — no hay que tocar ningún otro archivo
5. Verificar: `go build ./... && go test ./...`

O usar el skill: `/add-analyzer mianalyzer`

### Agregar un nuevo monitor

1. Crear `internal/monitor/mimonitor.go`
2. Implementar la interfaz `Monitor` (Name, Snapshot, Display)
3. Agregar cobra command en `cmd/monitors.go`
4. Agregar entrada en `cmd/menu.go` (menú lineal) y, para la TUI, una acción en `internal/tui/actions.go` (`buildActions`)

O usar el skill: `/add-monitor mimonitor`

### El monitor/analyzer se expone automáticamente en MCP

Los nuevos analyzers se registran via `init()` y el MCP server usa `analyzer.All()`, así que cualquier analyzer nuevo está disponible automáticamente en `mac_analyze` sin cambios adicionales.

### Build y test

```bash
make all        # lint + test + build
make test       # go test ./...
make build      # go build -o bin/toolkit
make lint       # go vet ./...
make install    # go build -o /usr/local/bin/toolkit
```

---

## Troubleshooting

### MCP server no arranca

```bash
# Verificar que el binario funciona
./bin/toolkit mcp --help

# Verificar ruta absoluta en la config del agente
cat .kiro/agents/mac-ops.json | grep command

# Rebuild si es necesario
go build -o bin/toolkit .
```

### Herramientas MCP no aparecen en Kiro

```bash
# Verificar config MCP
kiro-cli mcp list

# Verificar en chat
# Escribe /mcp para ver servers y tools
```

### Analyzer timeout

Si un analyzer tarda mucho, el runner lo cancela a los 180 segundos y retorna un resultado con `error: "timeout"`. Los otros analyzers no se ven afectados.

### Permisos en paths del sistema

Algunos paths como `/var/log` o `/Library/Logs` requieren permisos elevados. El toolkit los ignora silenciosamente si no tiene acceso.

### Tests fallan

```bash
go test ./... -v -count=1   # verbose para ver detalles
go test ./internal/core/... -run TestRunner  # test específico
```

---

## Arquitectura

```
mac-toolkit/
├── main.go                    # Entry point
├── cmd/                       # Cobra CLI
│   ├── root.go               # Root command + verbose flag
│   ├── tui.go                # comando tui (default) + comando menu
│   ├── menu.go               # menú lineal promptui
│   ├── doctor.go             # doctor (--no-ai, --json)
│   ├── docker.go             # docker backup/cleanup/compact
│   ├── analyze.go            # analyze + runAnalysis() + saveReports()
│   ├── clean.go              # clean + approval flow
│   ├── full.go               # full (analyze + save + clean)
│   ├── status.go             # status (dominios + riesgos)
│   ├── report.go             # report --last
│   ├── monitors.go           # battery, system, memory, cpu, processes, network
│   └── mcp.go                # MCP server (12 tools, stdio transport)
├── internal/
│   ├── core/                  # Fundamentos
│   │   ├── config.go         # Paths, thresholds, blacklists
│   │   ├── models.go         # CleanableItem, AnalysisResult, types
│   │   ├── runner.go         # Parallel execution (goroutines)
│   │   ├── logger.go         # ANSI styled output
│   │   ├── approval.go       # 4-mode approval engine
│   │   └── affirmative.go    # IsAffirmative: gate "si" compartido CLI/TUI
│   ├── analyzer/              # 11 domain analyzers
│   │   ├── analyzer.go       # Interface + DirSize, FileAge, MakeResult
│   │   ├── registry.go       # Global registry + Register/All/ByDomain
│   │   └── *.go              # disk, ollama, docker, browser, logs,
│   │                         # downloads, appsupport, repos, dev_caches,
│   │                         # xcode, trash
│   ├── monitor/               # 4 system monitors
│   │   ├── monitor.go        # Interface
│   │   ├── thermal.go        # parseThermal + IsThermalAlert
│   │   └── *.go              # battery, system, memory, cpu, processes, network
│   ├── tui/                   # TUI Bubble Tea (2 paneles, clean, doctor)
│   ├── ai/                    # doctor: snapshot sin PII, heurístico, Groq, collect
│   ├── docker/                # backup, cleanup, compact
│   ├── cleaner/               # Cleanup engine
│   │   ├── cleaner.go        # Interface + blacklist + Delete()
│   │   └── generic.go        # GenericCleaner
│   └── reporter/              # Output formats
│       ├── reporter.go       # Interface
│       └── *.go              # terminal, markdown, json, audit
├── .kiro/                     # Kiro integration
│   ├── agents/               # mac-ops, mac-toolkit-dev
│   ├── skills/               # 6 skills
│   └── steering/             # Project conventions
└── docs/                      # requirements, design, tasks, SAFETY,
                               # RELEASE_WORKFLOW, specs/, guías MCP
```

---

## Flujo de contribución y release

Cada cambio va por PR (`git switch -c <tipo>/<nombre>` → PR → CI verde → merge
squash); nunca a `main` directo. Una versión se publica con un tag `vX.Y.Z` sobre
`main` que dispara GoReleaser (binarios darwin arm64/amd64). Los PRs solo de
documentación se mergean sin tag ni release. Detalle y checklist en
[`docs/RELEASE_WORKFLOW.md`](docs/RELEASE_WORKFLOW.md).

---

*Mac Toolkit v1.8 — Go CLI + TUI + `doctor` + MCP para macOS*
