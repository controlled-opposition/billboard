{
  description = "bb, a command-line client for William & Mary's Blackboard Learn Ultra";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGoModule {
          pname = "bb";
          version = "0.1.0-${self.shortRev or self.dirtyShortRev or "unknown"}";
          src = self;
          vendorHash = "sha256-RlDXPViGCwVa3YYLdVNtTjsrrcEiB7G9/xvajsF6e30=";
          env.CGO_ENABLED = 0;
          ldflags = [
            "-s"
            "-w"
          ];
          meta = {
            description = "Command-line client for William & Mary's Blackboard Learn Ultra";
            mainProgram = "bb";
          };
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
          ];
          shellHook = ''
            git config core.hooksPath .githooks
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
