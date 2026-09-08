# Release & Contribution Workflow

Proceso estándar para cambios en `mac-toolkit`: cada cambio se entrega vía **Pull
Request**, se mergea a `main`, y se publica una **nueva versión de la CLI** con un
tag semver que dispara GoReleaser. Esta guía cubre el flujo completo de principio a
fin, incluyendo cómo actualizar el binario en local.

> Repo: `arheanja-ops/mac-toolkit` · rama base: `main` · versionado: [SemVer](https://semver.org) (`vX.Y.Z`)

---

## Resumen del flujo

```
feature branch → PR → CI (vet·test·build) → merge a main
      → tag vX.Y.Z → push tag → Release workflow (GoReleaser)
      → GitHub Release con binarios darwin arm64/amd64
      → actualización local (go install | brew | tar.gz | make)
```

| Etapa | Qué corre | Dónde |
|-------|-----------|-------|
| PR abierto/actualizado | `go mod tidy` check · `go vet` · `go test -race` · `go build` | `.github/workflows/ci.yml` (macos-latest) |
| Tag `v*.*.*` pusheado | GoReleaser build + publish | `.github/workflows/release.yml` |

---

## 1. Desarrollo en un feature branch

Nunca se commitea directo a `main`. Cada cambio vive en su propia rama.

```bash
git switch main
git pull --ff-only origin main

# Nombre descriptivo: feat/, fix/, docs/, refactor/, chore/
git switch -c feat/mi-cambio
```

Desarrolla y verifica localmente **antes** de abrir el PR (mismo set que corre CI):

```bash
make lint    # go vet ./...
make test    # go test ./...
make build   # go build -o bin/toolkit

# CI también valida que go.mod esté tidy — hazlo tú primero:
go mod tidy
git diff --exit-code go.mod go.sum   # debe salir limpio
```

Commits con [Conventional Commits](https://www.conventionalcommits.org) — GoReleaser
los usa para el changelog y filtra `docs:`, `test:`, `chore:`:

```bash
git add <archivos-específicos>        # evita `git add .`
git commit -m "feat: mejora el menú interactivo con banner y barras"
```

---

## 2. Crear el Pull Request

```bash
git push -u origin feat/mi-cambio

# Con GitHub CLI:
gh pr create \
  --base main \
  --title "feat: mejora del menú y presentación de datos" \
  --body "$(cat <<'EOF'
## Resumen
- Qué cambia y por qué.

## Cambios
- Punto 1
- Punto 2

## Pruebas
- make lint / test / build en verde
- Verificación manual: <comando y resultado>
EOF
)"
```

Reglas del PR:
- Título conciso (< 70 chars), detalle en la descripción.
- Estructura: resumen · cambios · qué se probó · features bloqueadas (si aplica).
- El PR **no se mergea hasta que CI esté verde** (job `vet · test · build`).

---

## 3. Merge a `main`

Una vez aprobado y con CI verde:

```bash
gh pr merge --squash --delete-branch
```

Squash mantiene el historial de `main` limpio (un commit por PR). Tras el merge,
sincroniza tu local:

```bash
git switch main
git pull --ff-only origin main
```

---

## 4. Publicar nueva versión de la CLI

El release se dispara **exclusivamente** al pushear un tag semver. Elige el bump
según el cambio (último tag: **`v1.1.0`**):

| Cambio | Bump | Ejemplo |
|--------|------|---------|
| Breaking / incompatibilidad | MAJOR | `v2.0.0` |
| Feature nueva compatible | MINOR | `v1.2.0` |
| Bugfix / interno | PATCH | `v1.1.1` |

```bash
# Asegúrate de estar en main actualizado y con CI verde.
git switch main
git pull --ff-only origin main

# Crea un tag anotado y púshalo.
git tag -a v1.2.0 -m "v1.2.0 — banner, menú y presentación de datos"
git push origin v1.2.0
```

Esto activa `.github/workflows/release.yml`, que corre GoReleaser (`release --clean`)
y produce en la [página de Releases](https://github.com/arheanja-ops/mac-toolkit/releases):

- Binarios `mac-toolkit` para **darwin arm64** (Apple Silicon) y **darwin amd64** (Intel).
- Archivos `mac-toolkit_<version>_darwin_<arch>.tar.gz`.
- `checksums.txt`.
- Changelog autogenerado desde los commits.
- Versión embebida en el binario vía ldflags (`cmd.version`, `cmd.commit`, `cmd.date`),
  visible con `mac-toolkit --version`.

Verifica que el release publicó bien:

```bash
gh run watch                         # sigue el workflow en vivo
gh release view v1.2.0               # confirma binarios y changelog
```

> Si el tag se pushó por error o falló, bórralo y reintenta:
> ```bash
> git push --delete origin v1.2.0    # remoto
> git tag -d v1.2.0                  # local
> ```
> (Operación destructiva sobre el remoto — confirma la versión antes de borrar.)

---

## 5. Actualizar la CLI en local

Tras publicar el release, actualiza tu instalación. Elige **una** opción:

### Opción A — `go install` (recomendada, más simple)

```bash
go install github.com/arheanja-ops/mac-toolkit@v1.2.0   # versión exacta
# o la última:
go install github.com/arheanja-ops/mac-toolkit@latest

mac-toolkit --version        # confirma la versión instalada
```

Queda en `$(go env GOPATH)/bin` — asegúrate de tenerlo en el `PATH`.

### Opción B — Descargar el binario del release

```bash
VERSION=v1.2.0
ARCH=arm64                   # arm64 (Apple Silicon) o amd64 (Intel)
BASE=https://github.com/arheanja-ops/mac-toolkit/releases/download/$VERSION

curl -fsSL -o mac-toolkit.tar.gz \
  "$BASE/mac-toolkit_${VERSION#v}_darwin_${ARCH}.tar.gz"

# (Opcional pero recomendado) verificar checksum
curl -fsSL -o checksums.txt "$BASE/checksums.txt"
shasum -a 256 -c checksums.txt --ignore-missing

tar -xzf mac-toolkit.tar.gz
sudo mv mac-toolkit /usr/local/bin/
mac-toolkit --version
```

### Opción C — Build desde el código fuente (para probar cambios locales sin release)

```bash
git switch main && git pull --ff-only origin main

make build                   # → bin/toolkit
./bin/toolkit --version

# Instalar globalmente:
make install                 # → /usr/local/bin/toolkit
```

> `make install` y los builds locales muestran versión `dev` (los ldflags de
> versión solo los inyecta GoReleaser en el release). Para probar una build local
> con datos reales de versión, usa la Opción A/B tras publicar el tag.

---

## Checklist rápido (copiar/pegar)

```bash
# 1. Rama
git switch main && git pull --ff-only origin main
git switch -c feat/mi-cambio

# 2. Verificar local (igual que CI)
go mod tidy && git diff --exit-code go.mod go.sum
make lint && make test && make build

# 3. PR
git add <archivos> && git commit -m "feat: ..."
git push -u origin feat/mi-cambio
gh pr create --base main --title "..." --body "..."

# 4. Merge (con CI verde)
gh pr merge --squash --delete-branch
git switch main && git pull --ff-only origin main

# 5. Release
git tag -a vX.Y.Z -m "vX.Y.Z — ..."
git push origin vX.Y.Z
gh run watch && gh release view vX.Y.Z

# 6. Actualizar local
go install github.com/arheanja-ops/mac-toolkit@vX.Y.Z
mac-toolkit --version
```
