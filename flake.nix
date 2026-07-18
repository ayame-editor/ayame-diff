{
  description = "ayame-diff development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      # The project cross-builds for Linux, macOS, and Windows, so the dev shell
      # covers the platforms a contributor is likely to be on rather than only
      # the maintainer's.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            # CI (lint.yml) runs gofmt, go vet, and staticcheck. gofmt and vet
            # ship with go; go-tools provides the staticcheck binary, so lint
            # findings surface before pushing. CI pins staticcheck and stays
            # authoritative; this one tracks nixpkgs and may differ.
            go-tools
            # The web assets are plain scripts with no build step; node is here
            # only to run their unit tests (node --test), matching CI.
            nodejs
          ];
        };
      });
    };
}
