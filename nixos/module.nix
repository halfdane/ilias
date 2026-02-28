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

    configFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to the ilias YAML configuration file.";
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

    nginx = {
      enable = lib.mkEnableOption "nginx virtual host for ilias";

      hostName = lib.mkOption {
        type = lib.types.str;
        default = "dashboard.localhost";
        description = "The hostname for the nginx virtual host.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
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
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      serviceConfig = {
        Type = "oneshot";
        User = cfg.user;
        Group = cfg.group;
        ExecStart = lib.concatStringsSep " " ([
          "${cfg.package}/bin/ilias"
          "generate"
          "-c" (toString cfg.configFile)
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
        locations."/" = {
          index = builtins.baseNameOf cfg.outputPath;
          tryFiles = "/${builtins.baseNameOf cfg.outputPath} =404";
        };
      };
    };
  };
}
