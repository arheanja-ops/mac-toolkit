# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-01
**Repo:** `arheanja-ops/mac-toolkit` · rama base `main` · último release **v1.5.0**

Lee este archivo al reiniciar para saber exactamente en qué punto quedamos.

---

## ✅ Entregado y en producción

| Versión | Qué entregó |
|---------|-------------|
| `v1.2.0` | UX: banner, menú menos saturado, mejor presentación de datos |
| `v1.3.0` | TUI estilo k9s (2 paneles + header en vivo) — Fase TUI-1 |
| `v1.3.1` | TUI: pantalla de bienvenida + acción Salir + fix del total |
| `v1.3.2` | Fix del timeout del analyzer `repos` (escaneo paralelo acotado) |
| `v1.4.0` | **Comando `doctor`** con IA (Groq) + fallback heurístico (Fase 1 IA) |
| `v1.5.0` | Rediseño profesional del TUI + gate de confirmación final en `clean` + `docs/SAFETY.md` |

Binario local actualizado a **v1.5.0** vía `go install`. (`--version` muestra `dev`
porque los ldflags solo los inyecta GoReleaser; el código SÍ es v1.5.0.)

---

## 🔒 Garantías de seguridad (ver docs/SAFETY.md)

- `doctor`, `analyze`, `status` y monitores: **solo lectura**.
- `clean`: dry-run por defecto → `--execute` → aprobación por modo → **confirmación
  final explícita** (escribir "si"); ante vacío/EOF, NO borra.
- TUI: solo lectura, garantizado por test (`TestBuildActionsAreReadOnly`).
- MCP: tools destructivas `dry_run=true` por defecto.
- Blacklist de rutas del sistema + audit log de toda eliminación.

---

## 🎯 PRÓXIMO PASO — roadmap pendiente

Specs en `docs/specs/`. Por prioridad sugerida:

1. **TUI-2 — Clean interactivo dentro del TUI**
   Checkboxes por item, color por riesgo, aprobación en pantalla — reutilizando
   los gates de seguridad de `cmd/clean.go` (dry-run + confirmación final).
2. **TUI-3 — Panel Doctor en el TUI**
   Enchufar el motor del `doctor` (ya existe en `internal/ai/`) como panel.
3. **Fase 2 IA — `doctor` agente (function-calling)**
   El modelo decide qué tools MCP invocar en loop. Depende de Fase 1 (hecha).
4. **Fase 3 IA — `optimize --safe` / `--performance`**
   Ejecutar quick wins seguros con una aprobación. Depende de Fase 1.
5. **Fase 4 (exploratoria)** — monitoreo recurrente en background (sin spec aún).

---

## ⚠️ Higiene pendiente del working tree

Hay cambios AJENOS (del IDE/Kiro, no nuestros) sin commitear, que hemos mantenido
INTACTOS y FUERA de todos nuestros PRs:

```
 M .kiro/agents/mac-ops.json
 M .kiro/agents/mac-toolkit-dev.json
```

Son reformateo JSON + reglas de permisos de los agentes. Decidir qué hacer con
ellos (commit aparte propio del proyecto, o descartar) antes de que estorben.
NUNCA mezclarlos con el trabajo de features.

---

## 📋 Flujo de trabajo (recordatorio)

Todo cambio: rama → PR → CI verde → merge squash a `main` → (si aplica) tag
`vX.Y.Z` → GoReleaser → `go install @vX.Y.Z`. Nunca commitear directo a `main`.
Siempre `git add` con rutas explícitas. Detalle: `docs/RELEASE_WORKFLOW.md`.
