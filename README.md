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

## Configuration reference

```yaml
title: My Dashboard
theme: dark          # "dark" (default) or "light"
refresh: 5m          # auto-reload interval; omit to disable

groups:
  - name: Group Name
    tiles:
      - name: Tile Name
        icon: /path/to/icon.png   # optional; local file or http(s) URL
        link: https://example.com # optional; makes the whole tile clickable

        generate:                 # optional; shell command run before rendering
          command: "some-tool --output /tmp/result.png"
          timeout: 30s            # default: 60s

        banner:                   # optional; full-width image shown below title/slots
          src: /tmp/result.png    # local file or http(s) URL
          type: image             # currently the only type; may be omitted

        slots:                    # optional; list of named status indicators
          - name: status
            default_status: { id: unknown, label: "❓" }   # used if the check fails to run
            check:
              type: http          # "http" or "command"
              target: https://example.com/health
              timeout: 10s        # optional
            rules:
              - match:
                  code: 200       # exact integer, or regex string: "2\\d\\d"
                  output: "ok"    # optional; regex matched against body / stdout+stderr
                status: { id: ok, label: "✅" }
              - match: {}         # catch-all — always place last
                status: { id: down, label: "🔴" }
```

### Local computer example

This config works out of the box on any standard Linux machine. Copy it, run `ilias generate`, and get an instant dashboard for your computer.

```yaml
title: My Computer
theme: dark
refresh: 1m

groups:
  - name: System
    tiles:
      - name: Hostname
        slots:
          - name: uptime
            check:
              type: command
              target: uptime -p    # standard on all Linux distros
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: error, label: "❌" }

      - name: Disk (root)
        slots:
          - name: usage
            check:
              type: command
              target: "df / --output=pcent | tail -1 | tr -d ' '"
            rules:
              - match:
                  output: "^[0-9]%$|^[0-6]\\d%$"
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

      - name: CPU temp
        slots:
          - name: temp
            check:
              type: command
              target: "sensors | awk '/^Package id 0:/ {print $4}'"   # lm-sensors
            rules:
              - match:
                  output: "^\\+[0-3]\\d"
                status: { id: ok, label: "✅ <40°C" }
              - match:
                  output: "^\\+[4-6]\\d"
                status: { id: warn, label: "⚠️ warm" }
              - match: {}
                status: { id: hot, label: "🔴 hot!" }

  - name: Network
    tiles:
      - name: Gateway
        slots:
          - name: ping
            check:
              type: command
              # ip and ping are standard on any Linux system
              target: "ping -c 1 -W 1 $(ip route show default | awk '/default/ {print $3; exit}')"
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: error, label: "❌" }

      - name: Internet
        link: https://example.com
        slots:
          - name: reachable
            check:
              type: http
              target: https://example.com
              timeout: 5s
            rules:
              - match:
                  code: 200
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: down, label: "🔴" }

  - name: Metrics
    tiles:
      # Requires prometheus-render (https://github.com/halfdane/prometheus-render)
      # and a local Prometheus instance. The generate command runs first, writes a
      # PNG, then the banner embeds it full-width in the tile.
      - name: CPU Usage
        banner:
          src: /tmp/ilias_cpu.png
          type: image
        generate:
          command: >-
            prometheus-render
            --query 'sum(rate(node_cpu_seconds_total{mode!="idle"}[5m])) * 100'
            --range 1h --title 'CPU %' --output /tmp/ilias_cpu.png
          timeout: 30s

      # Combine generate + banner + slots on one tile
      - name: Disk I/O
        banner:
          src: /tmp/ilias_diskio.png
          type: image
        generate:
          command: >-
            prometheus-render
            --query 'rate(node_disk_read_bytes_total[5m]) + rate(node_disk_written_bytes_total[5m])'
            --range 1h --title 'Disk I/O (bytes/s)' --output /tmp/ilias_diskio.png
          timeout: 30s
        slots:
          - name: read
            check:
              type: command
              target: "cat /sys/block/sda/stat | awk '{print $3*512}' | numfmt --to=iec"  # util-linux
            rules:
              - match:
                  code: 0
                status: { id: ok, label: "✅" }
              - match: {}
                status: { id: error, label: "❌" }
```

## CLI

```
ilias generate -c config.yaml -o output.html [-v]
ilias version
```

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | — | Path to the YAML config file (required) |
| `-o` | — | Output HTML file path (required) |
| `-v` | false | Verbose logging to stderr |

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
