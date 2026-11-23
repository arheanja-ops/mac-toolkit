# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-20
**Repo:** `arheanja-ops/mac-toolkit` · rama `main` · último release **v1.7.0**

## ▶ RETOMAR AQUÍ

1. **Bug a corregir (PATCH v1.7.1): el estado térmico "Warning" es un falso positivo.**
   `internal/monitor/system.go` (~línea 82) busca el texto `"No thermal warnings"`,
   pero `pmset -g therm` responde `"No thermal warning level has been recorded"`.
   Nunca coincide, así que SIEMPRE reporta `Warning`. Afecta: header de la TUI
   (🌡), insight "Proceso caliente" del `doctor` y `mac_system` del MCP.
   Fix: detectar ausencia de aviso con `"No thermal warning level"` (y cubrir ambos
   textos), con test de tabla sobre las dos salidas de `pmset`.
2. **`--version` muestra `dev`** con `go install`: leer la versión de
   `runtime/debug.ReadBuildInfo()` cuando los ldflags no estén inyectados.
3. **Higiene:** `gofmt -l` marca archivos previos sin formatear (`internal/analyzer/*`,
   `internal/docker/ops.go`, `internal/tui/helpers_test.go`) → PR de formato puro.
4. **`.kiro/agents/*.json` y `*.bak`:** cambios ajenos del IDE sin commitear; decidir
   commit propio o descartar. Nunca mezclarlos con features.

## 🧾 Tema en curso (no es del repo): ticket de cambio de laptop

El usuario prepara un ticket a IT (Jira Service Management → Hardware) pidiendo
reemplazo de su MacBook Pro M1 Pro (16 GB). Estado:
- Archivo de evidencia generado: `~/evidencia-rendimiento-20261120-1738.txt`
  (load average ~146/139/141 en 8 núcleos, swap 17.0 de 18.4 GB, vm_stat, Docker).
- **Pendiente:** el usuario toma 3–4 capturas de VENTANA (Monitor de Actividad:
  Memoria y CPU; Ajustes → Acerca de; opcional Almacenamiento), con su entorno de
  trabajo abierto. Luego se rehace el ticket con las cifras de ese mismo momento.
- **No usar** el térmico "Warning" como argumento (falso positivo, ver arriba) ni
  adjuntar la salida de `doctor` hasta corregirlo.
- Argumentos sólidos: saturación sostenida de CPU (load ≫ núcleos), thrashing de
  swap (~1.4 TiB out / ~1.4 TiB in desde el arranque), compresor de 7 GB, VM de
  Docker limitada a 2 CPU / 3.8 GB, flujo en paralelo (Docker, VPN, EKS, kiro-cli,
  Claude Code, Chrome). Punto débil: disco (hay ~18 GB de imágenes Docker y ~30 GB
  de cachés recuperables). La VPN pesa poco: argumentar flujo, no consumo.

## ✅ Entregado y en producción

| Versión | Qué entregó |
|---------|-------------|
| v1.2.0 | UX: banner, menú, presentación de datos |
| v1.3.0–1.3.2 | TUI k9s (2 paneles, header vivo) + bienvenida/Salir + fix total + fix timeout repos |
| v1.4.0 | Comando `doctor` con IA (Groq) + fallback heurístico |
| v1.5.0 | Rediseño del TUI + gate de confirmación final en clean + SAFETY.md |
| v1.6.0 | TUI-2: clean interactivo (checkboxes + modal "si", solo items safe) |
| v1.6.1 | TUI responsivo en 80 columnas + feedback "✓ Listo" |
| v1.7.0 | TUI-3: panel Doctor en el TUI (atajo `d`, `r` refresca) + `internal/ai/collect.go` |

## 🎯 Roadmap pendiente (por blast radius creciente)

- Limpiar `warn`/`danger` desde la TUI (TUI-2.1) con confirmación por item.
- Acciones Docker desde la TUI.
- Fase 2 IA: `doctor` agente (function-calling). Fase 3 IA: `optimize --safe`.

## 🔒 Seguridad (ver docs/SAFETY.md)

`doctor`, `analyze` y monitores: solo lectura. `clean`: dry-run por defecto → `--execute`
→ aprobación → confirmación "si". TUI: solo borra items `safe`, con modal "si", blacklist
y audit log. MCP: tools destructivas con `dry_run=true` por defecto.

## 📋 Flujo (recordatorio)

rama → PR → CI verde → merge squash → tag vX.Y.Z → release → `go install @vX.Y.Z`.
Nunca a main directo. `git add` con rutas explícitas. Detalle: docs/RELEASE_WORKFLOW.md.
