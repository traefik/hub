{
  description = "A very basic flake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }: 
  flake-utils.lib.eachDefaultSystem (system: let
    pkgs = import nixpkgs {
      inherit system;
    };
  in {
      devShells.default = pkgs.mkShell {
        shellHook = ''
        '';
        packages = with pkgs; [
          google-cloud-sdk
          k3s
          kube3d
          kubectl
          jq
          kubernetes-helm
          envsubst
        ];
      };
  });
}
