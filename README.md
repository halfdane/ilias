# ilias

A static HTML dashboard generator for self-hosted services.

Inspired by [Homer](https://github.com/bastienwirtz/homer) — where Homer serves a beautiful dashboard from a static directory, **ilias takes the idea further**: it actively runs checks (HTTP requests, shell commands) and bakes their results directly into a self-contained HTML file. No JavaScript runtime, no server process, no JavaScript framework. The output is a single `.html` file you can open in a browser or serve with nginx.

## Features

- **Groups and tiles** — organise services into named groups, each with an optional icon and link
- **HTTP checks** — poll any URL and match against status code and/or response body
- **Command checks** — run any shell command and match against exit code and/or stdout+stderr output
- **Flexible match rules** — exact integer matches or regex patterns; catch-all rules for defaults
- **Default status** — fallback status when a check cannot run at all
- **Generate commands** — run a command before rendering (e.g. to produce a chart image)
- **Banners** — embed a full-width image inside a tile (e.g. a Prometheus graph)
- **Tooltips** — hover over any status slot to see the raw check output
- **Auto-refresh** — configurable page-reload interval
- **Dark and light themes**
- **NixOS module** — systemd timer + optional nginx virtualhost, zero boilerplate

## Quick start

```sh
ilias generate -c config.yaml -o /tmp/dashboard.html
```

Open `/tmp/dashboard.html` in a browser. Re-run the command to refresh.

Or drop a file named `config.yaml` in the current directory and run `ilias generate` with no arguments — it writes `index.html` by default.

## Configuration

Save as `config.yaml` and run `ilias generate`. The example below works out of the box on most Linux machines and demonstrates every feature.

```yaml
title: My Computer
theme: dark    # "dark" (default) or "light"
refresh: 1m   # auto-reload interval; omit to disable

groups:
  - name: System
    tiles:
      - name: Uptime
        # icon: optional local file or http(s) URL embedded as a data URI
        # link: optional URL — makes the whole tile clickable
        slots:
          - name: uptime
            check:
              type: command         # "command" or "http"
              target: uptime        # shell command; stdout+stderr available in rules
            rules:
              - match:
                  code: 0           # exact integer exit code
                status: { id: ok, label: "✅" }
              - match: {}           # catch-all — always place last
                status: { id: error, label: "❌" }

      - name: Disk (root)
        slots:
          - name: usage
            check:
              type: command
              target: "df / --output=pcent | tail -1 | tr -d ' '"
            rules:
              - match:
                  output: "^[0-6]\\d%$|^[0-9]%$"  # regex on stdout+stderr
                status: { id: ok, label: "✅ <70%" }
              - match:
                  output: "^[7-8]\\d%$"
                status: { id: warn, label: "⚠️ 70–89%" }
              - match: {}
                status: { id: critical, label: "🔴 ≥90%" }

      - name: Memory
        slots:
          - name: available
            check:
              type: command
              target: "free -h | awk '/^Mem:/ {print $7 \" free\"}'"  # procps
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: error, label: "❌" }

  - name: Network
    tiles:
      - name: Gateway
        slots:
          - name: ping
            check:
              type: command
              target: "ping -c 1 -W 1 google.com"
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: down, label: "🔴" }

      - name: example.com
        link: https://example.com
        slots:
          - name: reachable
            check:
              type: http
              target: https://example.com
              timeout: 5s
            rules:
              - match:
                  code: 200               # exact HTTP status code
                  output: "Example Domain" # optional: regex on response body
                status: { id: ok, label: "✅" }
              - match:
                  code: "5\\d\\d"         # regex on status code
                status: { id: error, label: "🔴 5xx" }
              - match:
                  output: "certificate|x509|tls"  # TLS/cert errors (no code — match on error message)
                status: { id: cert-error, label: "🔒 cert error" }
              - match: {}
                status: { id: down, label: "🔴" }

  - name: Metrics
    tiles:
      # generate runs a shell command before rendering.
      # banner embeds a full-width image in the tile.
      - name: Some generated image
        banner:
          src: /tmp/generated_image1.svg
        generate:
          # Some command that actually generates the image. 
          # This is just an example!
          command: >-
            printf '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 32"><defs><linearGradient id="g"
            x1="0" y1="0" x2="1" y2="0"><stop offset="0%%" stop-color="#2ecc71"/><stop offset="60%%"
            stop-color="#f1c40f"/><stop offset="100%%" stop-color="#e74c3c"/></linearGradient></defs><rect
            width="400" height="32" rx="6" fill="url(#g)"/></svg>' > /tmp/generated_image1.svg
          timeout: 5s

      # generate + banner + slots can all be combined on one tile
      - name: Another generated image
        banner:
          src: /tmp/generated_image2.svg
        generate:
          command: >-
            printf '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 32"><defs><linearGradient id="g"
            x1="0" y1="0" x2="1" y2="0"><stop offset="0%%" stop-color="#3498db"/><stop offset="100%%"
            stop-color="#9b59b6"/></linearGradient></defs><rect width="400" height="32" rx="6"
            fill="url(#g)"/></svg>' > /tmp/generated_image2.svg
          timeout: 5s
        slots:
          - name: io
            check:
              type: command
              target: "vmstat 1 2 | tail -1 | awk '{print \"bi=\" $9 \" bo=\" $10 \" kB/s\"}'"
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: error, label: "❌" }
```

## CLI

```
ilias <command> [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `generate` | Run all checks and write the static HTML dashboard |
| `validate` | Parse and validate the config file without running any checks |
| `version` | Print the version and exit |

### `generate` flags

| Flag | Default | Description |
|------|---------|-------------|
| `-c`, `--config` | `config.yaml` | Path to the YAML config file |
| `-o`, `--output` | `index.html` | Output HTML file path |
| `--dry-run` | false | Print what would be checked without executing anything |
| `--concurrency` | auto (NumCPU) | Maximum number of parallel checks |
| `-v`, `--verbose` | false | Log progress and results to stderr |

### `validate` flags

| Flag | Default | Description |
|------|---------|-------------|
| `-c`, `--config` | `config.yaml` | Path to the YAML config file |
| `-v`, `--verbose` | false | Verbose output |

**Examples:**

```sh
# Minimal — reads config.yaml, writes index.html
ilias generate

# Explicit paths
ilias generate -c /etc/ilias/config.yaml -o /var/www/dashboard/index.html

# Preview what checks would run, without executing them
ilias generate --dry-run -c config.yaml

# Validate a config file
ilias validate -c config.yaml

# Print version
ilias version
```

## NixOS module

ilias ships a NixOS module. Add the flake as an input and import the module:

**flake.nix**

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    ilias.url   = "github:halfdane/ilias";
  };

  outputs = { nixpkgs, ilias, ... }: {
    nixosConfigurations.mymachine = nixpkgs.lib.nixosSystem {
      modules = [
        ilias.nixosModules.default
        ./configuration.nix
      ];
    };
  };
}
```

**configuration.nix**

```nix
services.ilias = {
  enable = true;

  # Point at a directory containing config.yaml and any local icon assets.
  # Nix copies the whole directory to the store, which resolves all relative paths.
  configDir = ./ilias;

  # Or, if you have no local assets, a single file is enough:
  # configFile = ./ilias/config.yaml;

  outputPath    = "/var/www/ilias/index.html";  # default
  timerInterval = "5min";                        # default

  # Extra packages made available on PATH for check and generate commands
  extraPackages = [ pkgs.jq pkgs.openssl ];

  nginx = {
    enable   = true;
    hostName = "dashboard.example.com";
    forceSSL = true;
    acmeHost = "example.com";  # reuse an existing ACME certificate
  };
};
```

### Module options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enable` | bool | — | Enable the service |
| `configDir` | path\|null | null | Directory with `config.yaml` and assets (takes precedence over `configFile`) |
| `configFile` | path\|null | null | Single config file path |
| `outputPath` | string | `/var/www/ilias/index.html` | Where to write the dashboard HTML |
| `timerInterval` | string | `5min` | Regeneration interval (systemd `OnUnitActiveSec` format) |
| `user` | string | `ilias` | System user for the service |
| `group` | string | `ilias` | System group for the service |
| `verbose` | bool | false | Enable verbose logging in the systemd service |
| `extraPackages` | list\<package\> | `[]` | Packages added to PATH for check and generate commands |
| `nginx.enable` | bool | false | Create an nginx virtual host |
| `nginx.hostName` | string | `dashboard.localhost` | Virtual host name |
| `nginx.forceSSL` | bool | false | Redirect HTTP → HTTPS |
| `nginx.acmeHost` | string\|null | null | Reuse an existing ACME certificate (`useACMEHost`) |

The service runs as a oneshot systemd unit triggered on boot (after 1 minute) and then on the configured interval. It also runs immediately on every `nixos-rebuild switch`.
