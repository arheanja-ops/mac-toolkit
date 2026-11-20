# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-11-02
**Repo:** `arheanja-ops/mac-toolkit` · rama `main` · último release **v1.6.1**

## ✅ Entregado y en producción

| Versión | Qué entregó |
|---------|-------------|
| v1.2.0 | UX: banner, menú, presentación de datos |
| v1.3.0–1.3.2 | TUI k9s (2 paneles, header vivo) + bienvenida/Salir + fix total + fix timeout repos |
| v1.4.0 | Comando `doctor` con IA (Groq) + fallback heurístico |
| v1.5.0 | Rediseño profesional del TUI + gate de confirmación final en clean + SAFETY.md |
| v1.6.0 | **TUI-2**: clean interactivo (checkboxes + modal "si", solo items safe) |
| v1.6.1 | Fix: TUI responsivo en 80 columnas + feedback "✓ Listo" en comandos |

Binario local en v1.6.1 (`--version` dice `dev` por lo de GoReleaser; el código es v1.6.1).

## 🎯 PRÓXIMO PASO (cuando vuelvas)
1. **PROBAR el TUI arreglado**: abrir `mac-toolkit` y confirmar que ya se ve bien en tu terminal.
2. Si algo sigue feo, ajustar; si está OK, seguir el roadmap:
   - TUI-3: panel Doctor dentro del TUI (atajo `d` reservado; motor ya existe en internal/ai/)
   - Limpiar warn/danger desde el TUI (TUI-2.1) con confirmación por item
   - Docker ops desde el TUI
   - Fase 2 IA (agente function-calling) · Fase 3 IA (optimize --safe/--performance)

## ⚠️ Higiene pendiente
`.kiro/agents/mac-ops.json` y `mac-toolkit-dev.json` tienen cambios AJENOS (del IDE)
sin commitear. Los hemos mantenido FUERA de todos los PRs, intactos (" M"). Decidir
qué hacer con ellos (commit propio o descartar) — nunca mezclarlos con features.

## 📋 Flujo (recordatorio)
rama → PR → CI verde → merge squash → tag vX.Y.Z → release → go install. Nunca a main
directo. git add con rutas explícitas. Detalle: docs/RELEASE_WORKFLOW.md.
