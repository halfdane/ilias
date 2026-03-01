{
  description = "ilias - static dashboard homepage generator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "aarch64-linux" "x86_64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default =
          let iliasVersion = "0.1.12"; in
          pkgs.buildGoModule {
            pname = "ilias";
            version = iliasVersion;
            src = ./.;
            vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";
            ldflags = [ "-X main.version=v${iliasVersion}" ];
            meta = {
              description = "Static dashboard homepage generator";
              mainProgram = "ilias";
            };
          };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools # staticcheck

            # README tooling
            (buildGoModule {
              pname = "embedmd";
              version = "1.0.0";
              src = fetchFromGitHub {
                owner = "campoy";
                repo = "embedmd";
                rev = "v1.0.0";
                hash = "sha256-hfMI2d3iRe74nUQ9ydgXUshStk9LFWXkJL1/7ZsEX6g=";
              };
              vendorHash = "sha256-uLhXMwnSHFUUiQlpDw/U6fZvNsRuB4cZhxX4qUtdknA=";
              doCheck = false;
            })
          ];
        };
      }
    ) // {
      nixosModules.default = import ./nixos/module.nix self;
    };
}
