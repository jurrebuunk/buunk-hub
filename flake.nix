{
  description = "buunk-hub: the people hub for buunk.org";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      # Everything to build, test and deploy without the cluster:
      # `nix develop`, then see README.md.
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.go
          pkgs.gopls
          pkgs.nodejs_22
          pkgs.kustomize
          pkgs.sops
          pkgs.ssh-to-age
        ];
        # Use the Go from this shell, never download another toolchain.
        shellHook = ''
          export GOTOOLCHAIN=local
        '';
      };
    };
}
