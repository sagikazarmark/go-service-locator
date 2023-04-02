{
  description = "Service locator ";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      rec
      {
        devShells = {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go-task
              go_1_20
            ];
          };

          ci = devShells.default;
        };
      });
}
