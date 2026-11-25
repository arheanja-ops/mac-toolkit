# 🔌 MCP Servers Setup — Kiro Crew

Configuración de MCP servers de uso general para Kiro Crew:
1. Kubernetes (oficial)
2. AWS (oficial awslabs)

---

## Prerequisitos

```bash
# Node.js (para npx)
node --version   # >= 18

# uv + uvx (para AWS MCP servers)
brew install uv
uvx --version

# kubectl configurado
kubectl get pods  # debe conectar a tu cluster
```

---

## 1. Kubernetes MCP Server

**Fuente**: [Flux159/mcp-server-kubernetes](https://github.com/Flux159/mcp-server-kubernetes) (1.6k ⭐)

### JSON para Kiro Crew → Add Custom:

```json
{
  "mcpServers": {
    "kubernetes": {
      "command": "npx",
      "args": ["mcp-server-kubernetes"]
    }
  }
}
```

### Con modo read-only (recomendado para empezar):

```json
{
  "mcpServers": {
    "kubernetes": {
      "command": "npx",
      "args": ["mcp-server-kubernetes"],
      "env": {
        "ALLOW_ONLY_NON_DESTRUCTIVE_TOOLS": "true"
      }
    }
  }
}
```

### Tools disponibles (~20):
- `kubectl_get` — Listar recursos (pods, deployments, services, etc)
- `kubectl_describe` — Describir un recurso
- `kubectl_logs` — Logs de un pod
- `kubectl_apply` — Aplicar YAML manifests
- `kubectl_create` — Crear recursos
- `kubectl_delete` — Borrar recursos (deshabilitado en read-only)
- `kubectl_scale` — Escalar deployments
- `kubectl_context` — Cambiar de contexto/namespace
- `kubectl_rollout` — Gestionar rollouts
- `explain_resource` — Explicar recursos K8s
- `port_forward` — Port forwarding a pods/services
- `install_helm_chart` / `upgrade_helm_chart` — Helm operations
- `cleanup_pods` — Limpiar pods problemáticos
- `node_management` — Cordon/drain/uncordon nodos
- `kubectl_generic` — Cualquier comando kubectl
- `ping` — Verificar conexión

### Ejemplos de prompts:
```
"¿Qué pods están corriendo en el namespace dev?"
"Muéstrame los logs del pod nginx-abc123"
"¿Cuántas réplicas tiene el deployment api-gateway?"
"¿Hay pods en CrashLoopBackOff?"
"Escala el deployment user-service a 3 réplicas"
"¿Qué ingresses hay en producción?"
```

---

## 2. AWS MCP Servers (oficial awslabs)

**Fuente**: [awslabs/mcp](https://github.com/awslabs/mcp) — 62 servers disponibles

AWS tiene muchos MCP servers especializados. Estos son los más relevantes para DevOps:

### a) AWS Documentation (docs + best practices):

```json
{
  "mcpServers": {
    "aws-docs": {
      "command": "uvx",
      "args": ["awslabs.aws-documentation-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### b) AWS CloudWatch (métricas, alarmas, logs):

```json
{
  "mcpServers": {
    "aws-cloudwatch": {
      "command": "uvx",
      "args": ["awslabs.cloudwatch-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### c) Amazon ECS (containers, servicios, tasks):

```json
{
  "mcpServers": {
    "aws-ecs": {
      "command": "uvx",
      "args": ["awslabs.ecs-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### d) Amazon EKS (Kubernetes en AWS):

```json
{
  "mcpServers": {
    "aws-eks": {
      "command": "uvx",
      "args": ["awslabs.eks-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### e) SNS + SQS (colas y notificaciones):

```json
{
  "mcpServers": {
    "aws-sns-sqs": {
      "command": "uvx",
      "args": ["awslabs.amazon-sns-sqs-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### f) AWS IAM (usuarios, roles, permisos):

```json
{
  "mcpServers": {
    "aws-iam": {
      "command": "uvx",
      "args": ["awslabs.iam-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### g) AWS Cost & Billing:

```json
{
  "mcpServers": {
    "aws-billing": {
      "command": "uvx",
      "args": ["awslabs.billing-cost-management-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### h) AWS CloudTrail (auditoría):

```json
{
  "mcpServers": {
    "aws-cloudtrail": {
      "command": "uvx",
      "args": ["awslabs.cloudtrail-mcp-server@latest"],
      "env": {
        "FASTMCP_LOG_LEVEL": "ERROR"
      }
    }
  }
}
```

### Catálogo completo de 62 servers:
Ver: https://awslabs.github.io/mcp/

### Ejemplos de prompts AWS:
```
"¿Cuáles son las alarmas activas de CloudWatch en eu-west-1?"
"Muéstrame los logs de la lambda user-auth de los últimos 30 minutos"
"¿Cuántos mensajes hay en la cola sqs payments-dlq?"
"¿Qué servicios ECS están corriendo en el cluster main?"
"¿Cuánto gastamos en AWS este mes?"
"¿Qué roles IAM tiene permisos sobre el bucket s3 config-prod?"
```

---

## Resumen — Servers en Kiro Crew

| Server | Command | Tools | Tipo |
|--------|---------|-------|------|
| `kubernetes` | `npx mcp-server-kubernetes` | ~20 (kubectl, helm, port-forward) | Stdio directo |
| `aws-docs` | `uvx awslabs.aws-documentation-mcp-server@latest` | Documentación AWS | Stdio directo |
| `aws-cloudwatch` | `uvx awslabs.cloudwatch-mcp-server@latest` | Métricas, alarmas, logs | Stdio directo |
| `aws-ecs` | `uvx awslabs.ecs-mcp-server@latest` | ECS services/tasks | Stdio directo |
| `aws-eks` | `uvx awslabs.eks-mcp-server@latest` | EKS clusters | Stdio directo |
| `aws-sns-sqs` | `uvx awslabs.amazon-sns-sqs-mcp-server@latest` | Colas y notificaciones | Stdio directo |
| `mac-toolkit` | `mac-toolkit mcp` | 12 (disk, battery, system, procs, net, status, clean, docker, doctor) | Stdio directo |

---

## Setup paso a paso en Kiro Crew

Para cada server:

1. **Kiro Crew** → **Agent Capabilities** → **Connections** → **MCP Servers**
2. Click **Add Custom**
3. Pegar el JSON correspondiente
4. Click **Apply**
5. Verificar que aparezca con status **Online** y el número de tools correcto

### Recomendación de orden:
1. Primero `kubernetes` (más útil para DevOps day-to-day)
2. Luego `aws-cloudwatch` + `aws-ecs` (observabilidad)
3. El resto de AWS según lo necesites

---

*Última actualización: 2026-11 (fila de `mac-toolkit` actualizada a v1.7.1: 12 tools)*

---

## 4. Mac Toolkit — borrados por MCP (opt-in)

Las tools destructivas de `mac-toolkit` (`mac_clean_batch`, `mac_docker_cleanup`)
rechazan `dry_run=false` salvo que el servidor arranque con
`MAC_TOOLKIT_MCP_ALLOW_DELETE=1`. Se declara en `env`, igual que los tokens de los
otros servidores:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"],
      "env": {
        "MAC_TOOLKIT_MCP_ALLOW_DELETE": "1"
      }
    }
  }
}
```

Además, cada borrado real exige `confirm_plan` igual al `plan_digest` devuelto por una
preview reciente, y escribe un audit log. Sin la variable, la configuración sin `env`
(solo lectura + preview) sigue funcionando. Detalle: `docs/SAFETY.md` (d).
