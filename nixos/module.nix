self:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.ilias;
  iliasPkg = self.packages.${pkgs.system}.default;
in
{
  options.services.ilias = {
    enable = lib.mkEnableOption "ilias static dashboard generator";

    package = lib.mkOption {
      type = lib.types.package;
      default = iliasPkg;
      description = "The ilias package to use.";
    };

    configDir = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = ''
        Directory containing the ilias config file and any local icon assets.
        The config file must be named <literal>config.yaml</literal> inside
        this directory. Icon paths in the config are resolved relative to it.

        When set to a path literal in your NixOS configuration, Nix copies the
        entire directory to the store, making all assets available at
        generation time. Example layout:

          ./ilias/
          ├── assets/
          │   └── logo.png
          └── config.yaml

        Takes precedence over <option>configFile</option> when set.
      '';
    };

    configFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = ''
        Path to the ilias YAML configuration file. Use
        <option>configDir</option> instead when you have local icon assets
        alongside the config.
      '';
    };

    outputPath = lib.mkOption {
      type = lib.types.str;
      default = "/var/www/ilias/index.html";
      description = "Path where the generated HTML dashboard will be written.";
    };

    timerInterval = lib.mkOption {
      type = lib.types.str;
      default = "5min";
      description = "How often to regenerate the dashboard (systemd OnUnitActiveSec format).";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "ilias";
      description = "User under which ilias runs.";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "ilias";
      description = "Group under which ilias runs.";
    };

    verbose = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Enable verbose logging.";
    };

    extraPackages = lib.mkOption {
      type = lib.types.listOf lib.types.package;
      default = [ ];
      example = lib.literalExpression "[ pkgs.openssl pkgs.jq ]";
      description = ''Additional packages to add to the PATH available to check commands.''
      ;
    };

    nginx = {
      enable = lib.mkEnableOption "nginx virtual host for ilias";

      hostName = lib.mkOption {
        type = lib.types.str;
        default = "dashboard.localhost";
        description = "The hostname for the nginx virtual host.";
      };

      forceSSL = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "Redirect HTTP to HTTPS on the virtual host.";
      };

      acmeHost = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = ''
          Use the ACME certificate for this hostname (sets
          <literal>useACMEHost</literal> on the virtual host). Implies
          <option>forceSSL</option> is likely desired.
        '';
      };
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.configDir != null || cfg.configFile != null;
        message = "services.ilias: either configDir or configFile must be set.";
      }
      {
        assertion = cfg.configDir == null || builtins.pathExists "${cfg.configDir}/config.yaml";
        message = "services.ilias: configDir is set but ${toString cfg.configDir}/config.yaml does not exist.";
      }
      {
        assertion = cfg.configFile == null || builtins.pathExists cfg.configFile;
        message = "services.ilias: configFile ${toString cfg.configFile} does not exist.";
      }
    ];

    users.users.${cfg.user} = lib.mkIf (cfg.user == "ilias") {
      isSystemUser = true;
      group = cfg.group;
    };

    users.groups.${cfg.group} = lib.mkIf (cfg.group == "ilias") { };

    systemd.tmpfiles.rules = [
      "d ${builtins.dirOf cfg.outputPath} 0755 ${cfg.user} ${cfg.group} -"
    ];

    systemd.services.ilias = {
      description = "ilias static dashboard generator";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      serviceConfig = {
        Type = "oneshot";
        User = cfg.user;
        Group = cfg.group;
        # Give commands in checks access to the full NixOS system PATH.
        # Systemd's default PATH only covers /usr/bin:/bin which is empty on NixOS.
        Environment = "PATH=${lib.makeBinPath cfg.extraPackages}:/run/current-system/sw/bin:/run/wrappers/bin";
        ExecStart = lib.concatStringsSep " " ([
          "${cfg.package}/bin/ilias"
          "generate"
          "-c" (if cfg.configDir != null
                then "${cfg.configDir}/config.yaml"
                else (toString cfg.configFile))
          "-o" cfg.outputPath
        ] ++ lib.optional cfg.verbose "-v");

        # Hardening
        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ReadWritePaths = [ (builtins.dirOf cfg.outputPath) ];
        ProtectHome = true;
        PrivateTmp = true;
      };
    };

    systemd.timers.ilias = {
      description = "Timer for ilias dashboard regeneration";
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnBootSec = "1min";
        OnUnitActiveSec = cfg.timerInterval;
        Unit = "ilias.service";
      };
    };

    services.nginx = lib.mkIf cfg.nginx.enable {
      enable = true;
      virtualHosts.${cfg.nginx.hostName} = {
        root = builtins.dirOf cfg.outputPath;
        forceSSL = cfg.nginx.forceSSL;
        useACMEHost = cfg.nginx.acmeHost;
        locations."/" = {
          index = builtins.baseNameOf cfg.outputPath;
          tryFiles = "/${builtins.baseNameOf cfg.outputPath} =404";
        };
      };
    };
  };
}
