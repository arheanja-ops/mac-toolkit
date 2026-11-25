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
      → actualización local (go install | binario del release | make)
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

El release se dispara **exclusivamente** al pushear un tag semver. Un PR solo de
documentación (`docs:`) se mergea sin tag ni release. Elige el bump
según el cambio (último tag: **`v1.7.1`**):

| Cambio | Bump | Ejemplo |
|--------|------|---------|
| Breaking / incompatibilidad | MAJOR | `v2.0.0` |
| Feature nueva compatible | MINOR | `v1.8.0` |
| Bugfix / interno | PATCH | `v1.7.2` |

```bash
# Asegúrate de estar en main actualizado y con CI verde.
git switch main
git pull --ff-only origin main

# Crea un tag anotado y púshalo.
git tag -a v1.8.0 -m "v1.8.0 — descripción breve del cambio"
git push origin v1.8.0
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
gh release view v1.8.0               # confirma binarios y changelog
```

> Si el tag se pushó por error o falló, bórralo y reintenta:
> ```bash
> git push --delete origin v1.8.0    # remoto
> git tag -d v1.8.0                  # local
> ```
> (Operación destructiva sobre el remoto — confirma la versión antes de borrar.)

---

## 5. Actualizar la CLI en local

Tras publicar el release, actualiza tu instalación. Elige **una** opción:

### Opción A — `go install` (recomendada)

```bash
go install github.com/arheanja-ops/mac-toolkit@latest      # o @vX.Y.Z

mac-toolkit --version        # versión; commit y fecha: none/unknown
```

Si quieres el binario con commit y fecha reales, usa la Opción B.

### Opción B — Binario del release (con commit y fecha)

```bash
VERSION=v1.9.0                 # versión a instalar (ver Releases)
ARCH=arm64                     # arm64 (Apple Silicon) o amd64 (Intel)
TMP=$(mktemp -d)

gh release download "$VERSION" --repo arheanja-ops/mac-toolkit \
  -p "*darwin_${ARCH}.tar.gz" -p checksums.txt -D "$TMP"

# Verificar el checksum antes de instalar (debe imprimir "OK")
(cd "$TMP" && grep "darwin_${ARCH}" checksums.txt | shasum -a 256 -c -)

tar -xzf "$TMP"/mac-toolkit_*_darwin_${ARCH}.tar.gz -C "$TMP"
DEST="$(go env GOPATH)/bin/mac-toolkit"            # o cualquier directorio de tu PATH
[ -f "$DEST" ] && cp -p "$DEST" "$DEST.prev"       # respaldo del binario actual
install -m 755 "$TMP/mac-toolkit" "$DEST"

mac-toolkit --version          # versión, commit y fecha reales
```

Requiere la GitHub CLI (`gh`).

### Opción C — Build desde el código fuente (para probar cambios locales sin release)

```bash
git switch main && git pull --ff-only origin main

make build                   # → bin/toolkit
./bin/toolkit --version

# Instalar globalmente:
make install                 # → /usr/local/bin/toolkit
```

> **Qué muestra `--version`** (`cmd/root.go`, `resolveBuildInfo`): los ldflags de
> versión, commit y fecha solo los inyecta GoReleaser en el release (Opción B).
> Con `go install ...@vX.Y.Z` (Opción A) ya se muestra la versión del tag, leída
> de `runtime/debug.ReadBuildInfo()`, pero el commit y la fecha quedan como
> `none`/`unknown` . Un build local en un checkout
> (Opción C) puede mostrar una versión derivada del VCS (p. ej. `v1.7.1+dirty`) o
> `dev`, según el toolchain. Para ver commit y fecha reales, usa el binario del
> release.

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

# 6. Actualizar local (go install, o binario del release + checksum; ver sección 5)
go install github.com/arheanja-ops/mac-toolkit@vX.Y.Z
gh release download vX.Y.Z --repo arheanja-ops/mac-toolkit -p '*darwin_arm64.tar.gz' -p checksums.txt -D "$TMP"
mac-toolkit --version
```
