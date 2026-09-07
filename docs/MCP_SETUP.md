# 🔌 Mac Toolkit MCP — Guía de Conexión

Guía para conectar el Mac Toolkit como MCP server en **Kiro Crew**, **Kiro Desktop (IDE)** y **Kiro CLI**.

---

## Tabla de Contenidos

1. [Resumen de herramientas MCP](#resumen-de-herramientas-mcp)
2. [Setup en Kiro Crew (App)](#setup-en-kiro-crew-app)
3. [Setup en Kiro Desktop (IDE)](#setup-en-kiro-desktop-ide)
4. [Setup en Kiro CLI (Terminal)](#setup-en-kiro-cli-terminal)
5. [Probando las herramientas](#probando-las-herramientas)
6. [Ejemplos de prompts por herramienta](#ejemplos-de-prompts-por-herramienta)
7. [Testing manual con JSON-RPC](#testing-manual-con-json-rpc)
8. [Troubleshooting](#troubleshooting)

---

## Resumen de herramientas MCP

El toolkit expone **7 herramientas** read-only (nunca borra archivos):

| # | Tool | Descripción | Parámetros |
|---|------|-------------|------------|
| 1 | `mac_analyze` | Análisis de disco (11 dominios en paralelo) | `domain` (opcional) |
| 2 | `mac_battery` | Batería: salud, ciclos, temperatura, voltaje | — |
| 3 | `mac_system` | CPU, memoria, swap, thermal, modelo/chip | — |
| 4 | `mac_processes` | Top 10 procesos por CPU y por memoria | — |
| 5 | `mac_network` | WiFi, tráfico, conexiones, online check | — |
| 6 | `mac_status` | Dominios registrados con nivel de riesgo | — |
| 7 | `mac_clean_preview` | Preview de limpieza (dry-run seguro) | `domain` (opcional) |

---

## Setup en Kiro Crew (App)

### Prerequisitos

1. El binario `toolkit` debe estar compilado en la máquina del usuario:
   ```bash
   cd mac-toolkit && go build -o bin/toolkit .
   ```

2. Anotar la ruta absoluta del binario:
   ```bash
   realpath bin/toolkit
   # → /Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit
   ```

### Paso 1: Agregar MCP Server en Kiro Crew

1. Abrir **Kiro Crew app**
2. Ir a **Agent Capabilities → Connections → MCP Servers**
3. Click **Add Custom**

### Paso 2: Configurar el JSON

Pegar esta configuración:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "/Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit",
      "args": ["mcp"]
    }
  }
}
```

> **Nota**: No necesita `MCP_TOKEN` ni bridge Node.js. A diferencia del Backstage MCP que usa un bridge HTTP, el Mac Toolkit es un MCP server nativo que habla JSON-RPC directo por stdio. Kiro Crew ejecuta el binario como proceso hijo.

### Paso 3: Apply

Click **Apply**. El server se conectará y las 7 herramientas estarán disponibles.

### Paso 4: Verificar

En el chat de Kiro Crew, las herramientas aparecerán como:
- `@mac-toolkit/mac_analyze`
- `@mac-toolkit/mac_battery`
- `@mac-toolkit/mac_system`
- `@mac-toolkit/mac_processes`
- `@mac-toolkit/mac_network`
- `@mac-toolkit/mac_status`
- `@mac-toolkit/mac_clean_preview`

### Comparación con Backstage MCP

| Aspecto | Backstage MCP | Mac Toolkit MCP |
|---------|--------------|-----------------|
| Transporte | HTTP bridge (Node.js) | Stdio directo (binario Go) |
| Auth | MCP_TOKEN requerido | No necesita (ejecución local) |
| Bridge | `backstage-mcp-bridge.js` | No necesita |
| Backend | Backstage API (localhost:7007) | Binario local |
| Runtime | Node.js + uvx | Binario Go (sin deps) |

---

## Setup en Kiro Desktop (IDE)

### Opción A: Global (todos los proyectos)

Editar `~/.kiro/settings/mcp.json`:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "/Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit",
      "args": ["mcp"]
    }
  }
}
```

> Si ya tienes otros servers (como `fetch`), agrégalo al mismo objeto `mcpServers`.

### Opción B: Por workspace

Crear `.kiro/settings/mcp.json` en el directorio del proyecto:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "/Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit",
      "args": ["mcp"]
    }
  }
}
```

### Verificar en Kiro Desktop

1. Escribe `/mcp` en el chat → debe aparecer `mac-toolkit` con status ✓
2. Si no aparece, abre **Command Palette** (⌘+Shift+P) → buscar "MCP" → "Reconnect MCP Servers"
3. Los cambios en `mcp.json` se detectan automáticamente (hot-reload, 500ms debounce)

---

## Setup en Kiro CLI (Terminal)

### Opción A: Usar el agente `mac-ops`

```bash
cd /Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit
kiro-cli chat --agent mac-ops
```

El agente ya tiene el MCP server configurado. Pregunta directamente:
```
> ¿cómo está mi mac?
> analiza el disco
> qué puedo limpiar
```

### Opción B: Agregar via CLI

```bash
kiro-cli mcp add \
  --name mac-toolkit \
  --command /Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit \
  --args mcp
```

### Opción C: Shortcut de teclado

Durante cualquier sesión de chat, presiona **Ctrl+Shift+M** para cambiar al agente `mac-ops`.

---

## Probando las herramientas

### Desde cualquier chat de Kiro (Crew, Desktop, o CLI)

Una vez conectado, estos son prompts naturales que activan las herramientas:

```
"¿Cómo está mi Mac?"
→ Usa: mac_system + mac_battery + mac_processes

"Analiza el disco completo"
→ Usa: mac_analyze (sin dominio = los 11)

"¿Cuánto espacio usan los caches de npm y brew?"
→ Usa: mac_analyze con domain=dev_caches

"¿Qué puedo limpiar de forma segura?"
→ Usa: mac_clean_preview

"¿Cuánto pesa Docker en mi disco?"
→ Usa: mac_analyze con domain=docker

"¿Qué procesos están consumiendo más CPU?"
→ Usa: mac_processes

"Revisa el estado de la batería"
→ Usa: mac_battery

"¿Estoy conectado a internet? ¿En qué WiFi estoy?"
→ Usa: mac_network

"Lista los dominios que puedes analizar"
→ Usa: mac_status
```

---

## Ejemplos de prompts por herramienta

| # | Herramienta | Ejemplo de prompt |
|---|-------------|-------------------|
| 1 | `mac_analyze` | "¿Cuánto espacio ocupan los caches de navegadores y qué puedo borrar?" |
| 2 | `mac_analyze` (domain) | "Analiza solo lo de Xcode — DerivedData y Simulators" |
| 3 | `mac_battery` | "¿Cuál es la salud de mi batería? ¿Cuántos ciclos tiene?" |
| 4 | `mac_system` | "¿Cuánta RAM tengo libre? ¿El CPU está alto?" |
| 5 | `mac_processes` | "¿Qué app está usando más memoria ahora mismo?" |
| 6 | `mac_network` | "¿A qué WiFi estoy conectado y cuál es la señal?" |
| 7 | `mac_status` | "¿Qué dominios de disco puede analizar el toolkit?" |
| 8 | `mac_clean_preview` | "Si limpio todo lo seguro, ¿cuánto espacio recupero?" |

---

## Testing manual con JSON-RPC

Para probar el MCP server directamente (útil para debugging):

### Listar herramientas

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | \
  ./bin/toolkit mcp
```

### Llamar mac_status

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mac_status","arguments":{}}}' | \
  ./bin/toolkit mcp
```

### Llamar mac_analyze (un dominio)

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mac_analyze","arguments":{"domain":"browser"}}}' | \
  ./bin/toolkit mcp
```

### Llamar mac_battery

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mac_battery","arguments":{}}}' | \
  ./bin/toolkit mcp
```

### Llamar mac_clean_preview

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mac_clean_preview","arguments":{}}}' | \
  ./bin/toolkit mcp
```

> **Nota**: El MCP protocol requiere primero un `initialize` handshake. Los comandos anteriores son para testing rápido. En producción, Kiro maneja el handshake automáticamente.

---

## Troubleshooting

### "El server no aparece en /mcp"

1. Verificar que el binario existe y funciona:
   ```bash
   /Users/jaime.henao/arheanja/scripts-hub/scripts-mac/mac-toolkit/bin/toolkit mcp --help
   ```
2. Verificar la ruta en el JSON — debe ser la ruta **absoluta**
3. En Kiro Desktop: ⌘+Shift+P → "Reconnect MCP Servers"
4. En Kiro Crew: re-apply la configuración del MCP server

### "Las herramientas no se ejecutan"

1. Verificar permisos del binario:
   ```bash
   chmod +x bin/toolkit
   ```
2. macOS puede bloquear binarios no firmados:
   ```bash
   xattr -d com.apple.quarantine bin/toolkit
   ```

### "mac_analyze tarda mucho"

Algunos dominios (como `repos`) escanean directorios grandes recursivamente. El timeout es 120s por analyzer. Los otros dominios no se ven afectados.

### "mac_battery retorna error"

En Macs de escritorio (iMac, Mac Mini, Mac Pro) no hay batería. El comando `ioreg -rn AppleSmartBattery` no retorna datos.

### "Necesito rebuild después de cambios"

```bash
cd mac-toolkit
go build -o bin/toolkit .
```

En Kiro Desktop el server se reconecta automáticamente al detectar cambios en `mcp.json`. Si cambiaste el binario pero no el JSON, reconecta manualmente.

### "Quiero agregar herramientas nuevas"

Edita `cmd/mcp.go`, agrega un nuevo `mcp.AddTool(...)` con su handler, rebuild, y las herramientas nuevas estarán disponibles automáticamente en el siguiente reconnect.

---

## Referencia rápida

```bash
# Build
cd mac-toolkit && go build -o bin/toolkit .

# CLI directa
./bin/toolkit analyze          # análisis
./bin/toolkit battery          # batería
./bin/toolkit system           # sistema
./bin/toolkit processes        # procesos
./bin/toolkit network          # red
./bin/toolkit status           # dominios
./bin/toolkit clean --execute  # limpiar (requiere terminal)

# MCP server
./bin/toolkit mcp              # iniciar server stdio

# Kiro CLI con agente
kiro-cli chat --agent mac-ops  # chat con herramientas MCP

# Kiro Desktop
# Agregar a ~/.kiro/settings/mcp.json y escribir /mcp para verificar

# Kiro Crew
# Agent Capabilities → Connections → MCP Servers → Add Custom
```

---

*Mac Toolkit MCP v1.0*
