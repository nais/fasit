{
  description = "Fasit";

  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = {nixpkgs, ...}: let
    goOverlay = final: prev: {
      go = prev.go.overrideAttrs (old: rec {
        version = "1.23.1";
        src = prev.fetchurl {
          url = "https://go.dev/dl/go${version}.src.tar.gz";
          hash = "sha256-buROKYN50Ual5aprHFtdX10KM2XqvdcHQebiE0DsOw0=";
        };
      });
    };
    withSystem = nixpkgs.lib.genAttrs ["x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin"];
    withPkgs = callback:
      withSystem (
        system:
          callback
          (import nixpkgs {
            inherit system;
            overlays = [goOverlay];
          })
      );
  in {
    devShells = withPkgs (pkgs: {
      default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
          gotools
          go-tools
          gnumake
          protobuf
        ];
      };
    });

    formatter = withPkgs (pkgs: pkgs.nixfmt-rfc-style);
  };
}
