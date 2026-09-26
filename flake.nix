{
  description = "go-version-auto-configure - detects and auto-fixes Go toolchain version-surface drift (go directives, go.work floors, Nix/CI pins) across the fleet";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    go-nix-helpers = {
      # Pinned 2026-09-22 to c42fd77 — same pin as file-and-image-renamer
      # (base-form pseudo-version normalization; revs below a97742e fail eval).
      url = "github:LarsArtmann/go-nix-helpers/c42fd7786ce1d188017c189151bbcb56aad12951";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{
      self,
      flake-parts,
      ...
    }:
    let
      lib = inputs.nixpkgs.lib;
      version = self.shortRev or self.dirtyShortRev or "dev";
      src = lib.fileset.toSource {
        root = ./.;
        fileset = lib.fileset.unions [
          ./.golangci.yml
          ./README.md
          ./go.mod
          ./go.sum
          ./cmd
          ./pkg
        ];
      };
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      go-standard = {
        pname = "go-version-auto-configure";
        inherit version src;
        vendorHash = "sha256-QBj4D9EQtEkzPWnVtg/KQi1c/oC5rZSxTPJS6u8JXTw=";
        description = "Detects and auto-fixes Go toolchain version-surface drift across the fleet";

        # ADR-0001: the fleet minor is 1.27 and go.mod declares `go 1.27`.
        # The pinned go-nix-helpers (c42fd77) defaults goPkgAttr to "go_1_26"
        # (the null auto-newest default landed after the pin), and the FOD
        # runs GOTOOLCHAIN=local — so the go-modules derivation died with
        # "go.mod requires go >= 1.27 (running go 1.26.7)" (2026-09-26).
        # Pin the branch explicitly; the locked nixpkgs ships go_1_27 = 1.27.1.
        goPkgAttr = "go_1_27";

        enableTempl = false;
        enableGoimports = false;
        enableGovulncheck = false;
        enableCheck = false;

        subPackages = [ "cmd/go-version-auto-configure" ];

        # All dependencies (go-finding, toolsdk, linter-autoconfigure-sdk,
        # golang.org/x/*) are public — fetched from the Go module proxy,
        # no flake inputs or local replaces needed.

        ldflags = [
          "-s"
          "-w"
          "-X github.com/larsartmann/go-version-auto-configure/pkg/version.injected=${version}"
        ];

        extraBuildAttrs.preBuild = ''
          export GOTOOLCHAIN=local
          export GOEXPERIMENT=jsonv2
        '';

        extraMeta = {
          homepage = "https://github.com/larsartmann/go-version-auto-configure";
          mainProgram = "go-version-auto-configure";
          platforms = lib.platforms.all;
        };

        devShellExtraPackages =
          pkgs: with pkgs; [
            go-tools
            gofumpt
            goimports-reviser
            revive
            deadnix
            golangci-lint
            go-licenses
          ];

        shellExtraEnv = {
          GOEXPERIMENT = "jsonv2";
          # Pin the exact toolchain so no shell needs GOTOOLCHAIN prefixes:
          # go_1_27 ships 1.27.1, and modules with a go 1.27 floor build as-is.
          GOTOOLCHAIN = "go1.27.1";
          # BuildFlow's shared devShell exports GOWORK=off globally, which
          # breaks `go work edit` (policy: workspace commands need workspace
          # discovery). Empty string means unset to the go command.
          GOWORK = "";
        };
      };
    };
}
