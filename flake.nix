{
  description = "A very basic flake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    hub-acp.url = "git+ssh://git@github.com/juliens/hub-acp?ref=nix-gomod";
    hub-admin.url = "git+ssh://git@github.com/juliens/hub-admin?ref=nix-gomod";
    hub-alert.url = "git+ssh://git@github.com/juliens/hub-alert?ref=nix-gomod";
    hub-api-management.url = "git+ssh://git@github.com/juliens/hub-api-management?ref=nix-gomod";
    #hub-api-management.url = "git+ssh://git@github.com/juliens/hub-api-management?ref=nix";
    hub-certificates.url = "git+ssh://git@github.com/juliens/hub-certificates?ref=nix-gomod";
    hub-check.url = "git+ssh://git@github.com/juliens/hub-check?ref=nix-gomod";
    hub-cluster.url = "git+ssh://git@github.com/juliens/hub-cluster?ref=nix-gomod";
    hub-gslb.url = "git+ssh://git@github.com/juliens/hub-gslb?ref=nix-gomod";
    hub-invitation.url = "git+ssh://git@github.com/juliens/hub-invitation?ref=nix-gomod";
    hub-mail.url = "git+ssh://git@github.com/juliens/hub-mail?ref=nix-gomod";
    hub-metrics.url = "git+ssh://git@github.com/juliens/hub-metrics?ref=nix-gomod";
    hub-notification.url = "git+ssh://git@github.com/juliens/hub-notification?ref=nix-gomod";
    hub-offer.url = "git+ssh://git@github.com/juliens/hub-offer?ref=nix-gomod";
    hub-token.url = "git+ssh://git@github.com/juliens/hub-token?ref=nix-gomod";
    hub-topology.url = "git+ssh://git@github.com/juliens/hub-topology?ref=nix-gomod";
    hub-user.url = "git+ssh://git@github.com/juliens/hub-user?ref=nix-gomod";
    hub-workspace.url = "git+ssh://git@github.com/juliens/hub-workspace?ref=nix-gomod";
  };

  outputs = { self, nixpkgs, flake-utils,... }@attrs: 
  flake-utils.lib.eachDefaultSystem (system: let
    pkgs = import nixpkgs {
      inherit system;
      config.allowUnfree = true;
    };
  in {
      devShells.deploy = pkgs.mkShell {
        shellHook = ''
          export WORKSPACE_ID="6311c90bfce04bd29e473a20"
          docker load < ${attrs.hub-acp.packages.${system}.image}
          docker load < ${attrs.hub-acp.packages.${system}.image}
          docker load < ${attrs.hub-admin.packages.${system}.image}
          docker load < ${attrs.hub-alert.packages.${system}.image}
          docker load < ${attrs.hub-api-management.packages.${system}.image}
          docker load < ${attrs.hub-certificates.packages.${system}.image}
          docker load < ${attrs.hub-check.packages.${system}.image}
          docker load < ${attrs.hub-cluster.packages.${system}.image}
          docker load < ${attrs.hub-gslb.packages.${system}.image}
          docker load < ${attrs.hub-invitation.packages.${system}.image}
          docker load < ${attrs.hub-mail.packages.${system}.image}
          docker load < ${attrs.hub-metrics.packages.${system}.image}
          docker load < ${attrs.hub-notification.packages.${system}.image}
          docker load < ${attrs.hub-offer.packages.${system}.image}
          docker load < ${attrs.hub-token.packages.${system}.image}
          docker load < ${attrs.hub-topology.packages.${system}.image}
          docker load < ${attrs.hub-user.packages.${system}.image}
          docker load < ${attrs.hub-workspace.packages.${system}.image}
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
