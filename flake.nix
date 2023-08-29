{
  description = "A very basic flake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    hub-api-management.url = "git+ssh://git@github.com/juliens/hub-api-management?ref=nix";
    hub-acp = {
      url = "git+ssh://git@github.com/juliens/hub-acp?ref=nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    hub-workspace.url = "git+ssh://git@github.com/juliens/hub-workspace?ref=nix";
  };

  outputs = { self, nixpkgs, flake-utils, hub-api-management, hub-acp, hub-workspace,... }: 
  flake-utils.lib.eachDefaultSystem (system: let
    pkgs = import nixpkgs {
      inherit system;
      config.allowUnfree = true;
    };
    #env_file = builtins.fromTOML ( builtins.readFile ./hub/.env );
    #github_token = if builtins.hasAttr "GITHUB_TOKEN" env_file then env_file.GITHUB_TOKEN else throw "You need to define GITHUB_TOKEN in ./hub/.env";
  in {
      devShells.default = pkgs.mkShell {
        shellHook = ''
          export WORKSPACE_ID="6311c90bfce04bd29e473a20"
          docker load < ${hub-api-management.packages.${system}.image}
          docker load < ${hub-acp.packages.${system}.image}
          docker load < ${hub-workspace.packages.${system}.image}
        '';
        packages = with pkgs; [
          google-cloud-sdk
          k3s
          kube3d
          kubectl
          jq
          kubernetes-helm
          envsubst

          mongodb-compass
          natscli

          yamllint
        ];
      };
  });
}
