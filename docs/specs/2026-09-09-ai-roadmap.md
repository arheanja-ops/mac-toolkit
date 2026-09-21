# AI & Agents Roadmap — mac-toolkit

**Date:** 2026-09-09
**Status:** Draft
**Scope:** mac-toolkit CLI + MCP server

## Visión

Hoy el toolkit es **descriptivo**: recolecta métricas de disco (11 analyzers) y
sistema (4 monitores) y las muestra en tablas. El siguiente paso es hacerlo
**prescriptivo** (interpreta lo que encuentra y recomienda) y luego **asistido**
(actúa con aprobación del usuario), apoyándose en IA vía Groq.

La data estructurada ya existe — analyzers y monitores devuelven structs. La IA
opera *encima* de esa data: recibe un snapshot y produce diagnóstico + plan
priorizado. No ejecuta comandos ni accede al sistema directamente, lo que la hace
segura, barata y rápida.

```
DATA (existe)            CAPA IA (nueva)              SALIDA
analyzers + monitors  →  internal/ai (Groq client)  →  toolkit doctor
   → snapshot JSON        + fallback heurístico          diagnóstico + plan
```

## Principios

- **Nunca dependiente de IA para funcionar.** Sin `GROQ_API_KEY`, `doctor` cae a un
  motor heurístico determinista. La CLI sigue siendo útil offline.
- **Privacidad primero.** Por defecto se envían agregados (tamaños por dominio),
  nunca rutas de repos ni nombres de archivo del usuario.
- **Seguridad heredada.** La IA recomienda; las acciones destructivas siguen
  pasando por el flujo de aprobación y blacklist existentes. La IA nunca borra
  directo.
- **Sin frameworks pesados.** Groq es OpenAI-compatible; basta `net/http` stdlib.
  Cada dependencia nueva se justifica.

## Fases

### Fase 1 — `doctor` (analista) · [spec](2026-09-09-ai-doctor.md)

Comando `toolkit doctor` que orquesta análisis + monitores, arma un snapshot
agregado y lo envía a Groq para producir un diagnóstico priorizado
(performance / quick wins / requiere revisión). Fallback heurístico sin API key.

**Entrega:** el 80% del valor. Base para las fases siguientes.
**Riesgo:** bajo — solo lectura, sin acciones.

### Fase 2 — `doctor` agente (function-calling)

Groq soporta tool-calling. El modelo decide qué tools del toolkit invocar
(`mac_analyze`, `mac_clean_preview`, `mac_docker_compact`…) en un loop
conversacional. La infraestructura de tools ya existe en el MCP server.

**Entrega:** interacción tipo chat standalone, sin cliente MCP externo.
**Riesgo:** medio — requiere límites de iteración y gating de tools destructivas.
**Depende de:** Fase 1 (cliente Groq, snapshot).

### Fase 3 — optimización asistida y perfiles

Perfiles de acción sobre el diagnóstico:
- `optimize --safe` → ejecuta solo quick wins de riesgo cero (caches, node_modules)
  con una sola aprobación.
- `optimize --performance` → foco en RAM/CPU/thermal: identifica procesos huérfanos,
  sugiere apps de login pesadas. Solo diagnóstico + sugerencias (no mata procesos
  sin confirmación explícita).

**Entrega:** cierra el ciclo descriptivo → prescriptivo → asistido.
**Riesgo:** medio/alto — toca acciones del sistema; requiere confirmación granular.
**Depende de:** Fase 1.

### Fase 4 (exploratoria) — monitoreo recurrente

Ejecución en background que avisa cuando algo se degrada (swap alto, disco >85%,
thermal warning sostenido). Fuera de alcance inmediato; se evalúa tras Fase 1-3.

## Configuración (transversal)

| Variable | Default | Propósito |
|----------|---------|-----------|
| `GROQ_API_KEY` | — (vacío → modo heurístico) | Autenticación Groq |
| `GROQ_MODEL` | `openai/gpt-oss-20b` | Modelo de inferencia |
| `GROQ_BASE_URL` | `https://api.groq.com/openai/v1` | Endpoint OpenAI-compatible |

> Los modelos `llama-3.1-8b-instant` y `llama-3.3-70b-versatile` fueron deprecados
> por Groq en jun-2026; por eso el default es `openai/gpt-oss-20b`. Verificar
> modelos vigentes en https://console.groq.com/docs/models antes de fijar.

## Orden de ejecución

Fase 1 primero (independiente y de bajo riesgo). Fases 2 y 3 dependen de la 1 y
pueden ir en paralelo. Cada fase se entrega por su propio PR → tag → release,
siguiendo [RELEASE_WORKFLOW.md](../RELEASE_WORKFLOW.md).
