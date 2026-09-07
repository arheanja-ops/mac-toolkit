#!/usr/bin/env bash
# Verifica la sesión de gh dentro del GH_CONFIG_DIR aislado de mac-toolkit (separado del gh del
# equipo de trabajo) y la inicia si hace falta.
# Uso: ./scripts/gh-login.sh
set -euo pipefail

if [ -z "${GH_CONFIG_DIR:-}" ]; then
  echo "⚠️  GH_CONFIG_DIR no está seteado — ¿corriste esto fuera de mac-toolkit/, o direnv no cargó?"
  echo "   Sin GH_CONFIG_DIR esto usaría la config de gh de trabajo por defecto."
  echo "   Solución: cd al repo y ejecuta 'direnv allow', luego reintenta."
  exit 1
fi

echo "→ Verificando sesión gh aislada (GH_CONFIG_DIR: ${GH_CONFIG_DIR})..."

if gh auth status >/dev/null 2>&1; then
  echo "✅ Sesión activa:"
  gh auth status
else
  echo "⏳ Sin sesión en este directorio aislado — abriendo el navegador para autorizar..."
  echo "   Usa tu cuenta personal (jaimehenao8126), no la de trabajo."
  gh auth login --hostname github.com --git-protocol ssh --web
  echo "✅ Login completado:"
  gh auth status
fi
