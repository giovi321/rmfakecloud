---
title: Building this fork
---

You do not have to build this yourself. Every push to `local-build` is built by the
`build-binary` workflow and published as a rolling prerelease on the `local-build` tag,
carrying three assets:

| Asset | What it is |
|-------|------------|
| `rmfakecloud-linux-amd64` | the binary, linux amd64, dynamically linked against libcairo |
| `rmfakecloud-linux-amd64.sha256` | its sha256, as the bare hex digest |
| `VERSION` | the version string compiled into the binary through `-ldflags` |

`VERSION` exists so that asking "is this release the one installed" costs one plain
download and is answered by reading the installed binary, rather than by trusting a
record of what was installed last time. The two differ exactly when somebody replaced
the binary by hand, which is the case worth catching.

The workflow runs on ubuntu-24.04 on purpose. Its glibc is 2.39 and the target's is
2.41, and a binary built against an older glibc runs against a newer one. The reverse
does not, so building on a newer distro than the target would produce something that
refuses to start.

## Installing the published build

```bash
BASE=https://github.com/giovi321/rmfakecloud/releases/download/local-build
curl -fsSL -o rmfakecloud "$BASE/rmfakecloud-linux-amd64"
want=$(curl -fsSL "$BASE/rmfakecloud-linux-amd64.sha256" | tr -d '\r\n' | cut -d' ' -f1)
[ "$want" = "$(sha256sum rmfakecloud | cut -d' ' -f1)" ] || { echo "sha mismatch"; exit 1; }
```

Then install it over the path the unit execs, as below.

## Building it yourself

Two paths, both producing the same binary. The docker path is upstream's `Dockerfile`
and needs nothing installed. The native path needs a toolchain but no docker.

Whichever you use, the binary is the deliverable. Building and restarting the service
without an install step in between restarts the old binary, reports the unit as active
and looks like a successful upgrade while nothing changed.

## Two traps that fail silently

**Build the UI before the binary.** `go generate ./...` embeds `ui/dist`. If that
directory is missing or stale, the stale copy is baked into the binary and nothing
later reports it.

**`pnpm install` prints a node-gyp failure for `canvas@2.11.2`.** It is expected and
harmless, the vite build does not use it, and treating it as fatal stops a build that
is fine. What matters is that `ui/dist` exists afterwards and is freshly dated.

## Native build

Requires Go per `go.mod` (1.25.1 or newer, tested with 1.27.1), Node LTS with pnpm
through corepack, and cairo development headers. On Debian that is `libcairo2-dev` and
`pkg-config`; Debian's packaged Go is too old, so install Go from a tarball.

```bash
git clone -b local-build https://github.com/giovi321/rmfakecloud
cd rmfakecloud

# the UI first, because go generate embeds ui/dist
( cd ui && pnpm install --frozen-lockfile && pnpm build )
ls -la ui/dist

# then the binary. -tags cairo with CGO enabled is what the v6 export needs:
# built without them the binary still runs and the export silently degrades.
go generate ./...
CGO_ENABLED=1 go build -tags cairo \
  -ldflags "-s -w -X main.version=local-build-$(git rev-parse --short HEAD)" \
  -o rmfakecloud ./cmd/rmfakecloud/
sha256sum rmfakecloud
```

## Docker build

Upstream's `Dockerfile` builds the UI in `node:lts`, the binary in `golang:bookworm`
with `CGO_ENABLED=1` and `-tags cairo`, and ships it on `debian:bookworm-slim`. Strip
the platform pin first so it builds natively rather than under emulation.

```bash
sed -e 's/FROM --platform=[^ ]* /FROM /' Dockerfile > Dockerfile.native
docker build -f Dockerfile.native --build-arg VERSION=local-build -t rmfakecloud:local .
```

To take the binary out of the image, read the entrypoint rather than assuming the path.
Images built from this fork's tree carry `/rmfakecloud`; images built from upstream
master carry `/rmfakecloud-docker`.

```bash
docker inspect --format '{{.Config.Entrypoint}}' rmfakecloud:local
c=$(docker create rmfakecloud:local)
docker cp "$c":/rmfakecloud ./rmfakecloud
docker rm "$c"
```

## Installing it over a running service

Derive the path from the unit rather than guessing it, keep the previous binary, and
confirm afterwards that the process is running the new one. A unit being active proves
only that something started, and a clean restart of the old binary is also active.

```bash
UNIT=rmfakecloud
new=./rmfakecloud
install_path=$(systemctl show "$UNIT" -p ExecStart | sed -n 's/.*path=\([^ ;]*\).*/\1/p')

cmp -s "$new" "$install_path" && { echo "identical, the build delivered nothing"; exit 1; }
cp -a "$install_path" "$install_path".prev

# stage on the same filesystem, then rename: mv across filesystems is a copy and can
# leave a truncated binary at the exec path if it is interrupted
tmp="$(dirname "$install_path")/.$(basename "$install_path").new.$$"
install -m 755 "$new" "$tmp" && mv -f "$tmp" "$install_path"

systemctl restart "$UNIT"
systemctl is-active "$UNIT"
cmp -s "$install_path" "$new" && echo "running binary matches the build"
```

Rolling back one build is a copy of `$install_path.prev` over the exec path and a
restart. Config, data and the sync protocol are untouched by these changes, so a
rollback needs no data restore.
