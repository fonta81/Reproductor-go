{
  description = "Devshell para proyectos Go (bubbletea + oto)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      # Define los sistemas compatibles (añade más si usas macOS u otras arquitecturas)
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      
      # Genera la configuración para cada sistema
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              pkg-config
              alsa-lib
            ];

            shellHook = ''
              echo "Entorno Go listo (bubbletea + oto)"
              go version
            '';
          };
        }
      );
    };
}
