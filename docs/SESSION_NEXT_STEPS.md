# 📍 DÓNDE VAMOS — Punto de retomada

**Última actualización:** 2026-09-09
**Repo:** `arheanja-ops/mac-toolkit` · rama base `main`

Lee este archivo al reiniciar para saber exactamente en qué punto quedamos.

---

## ✅ Hecho y publicado

- **`v1.2.0` — UX de la CLI** (PR #1, mergeado y released):
  - Banner ASCII de inicio con versión + barra de uso de disco en vivo.
  - Menú menos saturado: secciones (Disk/Monitors/Reports) + acciones con
    descripción alineada.
  - Presentación de datos mejorada: dominios/items ordenados por tamaño, barras de
    proporción, top-8 por dominio con resumen del resto, checkmarks de color.
  - Instalado en local vía `go install ...@v1.2.0` (banner funcionando).
    Nota: `--version` muestra `dev` con `go install` — los ldflags de versión solo
    los inyecta GoReleaser (usar el tar.gz del release si se necesita la versión real).

- **Proceso de release documentado:** `docs/RELEASE_WORKFLOW.md`
  (flujo PR → merge → tag `vX.Y.Z` → GoReleaser → actualización local).

---

## 📝 Diseñado, SIN implementar, SIN commitear

Specs de la integración de IA con Groq (creadas en el working tree, aún no en git):

- `docs/specs/2026-09-09-ai-roadmap.md` — roadmap de 4 fases.
- `docs/specs/2026-09-09-ai-doctor.md` — spec detallada de la Fase 1.
- `docs/SESSION_NEXT_STEPS.md` — este archivo.

**Estado git:** estos 3 archivos están como *untracked*. Aún no hay PR de las specs.

---

## 🎯 PRÓXIMO PASO al retomar

**Decisión pendiente del usuario:** ¿subimos las specs por PR y/o arrancamos la
implementación de la Fase 1?

### Opción A — Subir las specs primero (recomendado)
PR pequeño solo con la documentación de diseño, siguiendo `RELEASE_WORKFLOW.md`:
```bash
git switch main && git pull --ff-only origin main
git switch -c docs/ai-roadmap-specs
git add docs/specs/2026-09-09-ai-roadmap.md docs/specs/2026-09-09-ai-doctor.md docs/SESSION_NEXT_STEPS.md
git commit -m "docs: roadmap de IA (Groq) y spec de la fase 1 (doctor)"
git push -u origin docs/ai-roadmap-specs
gh pr create --base main --title "docs: roadmap de IA y spec del comando doctor" --body "..."
# CI verde → gh pr merge --squash --delete-branch
```
(Solo docs → no requiere nuevo tag/release.)

### Opción B — Implementar la Fase 1 (`doctor`)
Arrancar el código según `docs/specs/2026-09-09-ai-doctor.md`. Bump esperado
`v1.3.0` (MINOR). Componentes a construir:
- `internal/ai/` — cliente Groq (net/http stdlib) + motor heurístico + AISnapshot.
- `cmd/doctor.go` — comando `toolkit doctor` (+ `--no-ai`, `--json`).
- Tool MCP `mac_doctor` en `cmd/mcp.go`.
- Tests: heurístico (tabla), cliente (httptest mock), redacción de key, privacidad.

---

## 🔑 Defaults acordados para la Fase 1

- **Config por entorno:** `GROQ_API_KEY` (vacío → modo heurístico),
  `GROQ_MODEL=openai/gpt-oss-20b`, `GROQ_BASE_URL=https://api.groq.com/openai/v1`.
- **Modelo:** `openai/gpt-oss-20b` (ojo: `llama-3.1-8b-instant` y
  `llama-3.3-70b-versatile` fueron deprecados por Groq en jun-2026).
- **Privacidad:** enviar solo agregados a Groq — sin rutas, sin nombres de archivo,
  sin PIDs.
- **Fallback:** sin API key → motor heurístico determinista, la CLI nunca depende
  de IA ni de internet.
- **Seguridad:** `doctor` es solo lectura; las acciones destructivas siguen pasando
  por el flujo de aprobación/blacklist existente.

---

## 🐛 Bug conocido pendiente (detectado en output real)

El dominio `disk` (uso total del volumen, ~346 GB) se suma al "Total" del resumen
de `analyze`, produciendo un total engañoso (443 GB) y barras de proporción
inútiles. La spec de `doctor` ya lo corrige a nivel de diagnóstico (lo trata como
"contexto", no como recuperable), pero **el reporter de `analyze` sigue con el bug**.
Candidato a fix aparte (`v1.2.1` PATCH) si se quiere corregir antes de la Fase 1.

---

## 📋 Flujo de trabajo establecido (recordatorio)

Todo cambio: rama → PR → CI verde → merge squash a `main` → (si aplica) tag
`vX.Y.Z` → GoReleaser → actualizar local. Nunca commitear directo a `main`.
Detalle completo en `docs/RELEASE_WORKFLOW.md`.
