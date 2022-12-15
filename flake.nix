{
  # This flake is a stub around default.nix 
  inputs.nixpkgs.url = "github:NixOS/nixpkgs";

  outputs = { nixpkgs, ... } @ inputs: let 
    eachSupportedSystemMap = fn: builtins.foldl' ( acc: system: acc // {
      ${system} = fn system;
    } ) {} [
      "aarch64-darwin"
      "x86_64-darwin"
      "aarch64-linux"
      "x86_64-linux"
    ];
    packages = eachSupportedSystemMap ( system: let 
      pkgsFor = nixpkgs.legacyPackages.${system};
      oplogtoredis   = pkgsFor.callPackage ./default.nix {};
    in { inherit oplogtoredis; default = oplogtoredis; } );
  in {
    inherit packages;
    defaultPackage =
      eachSupportedSystemMap ( system: packages.${system}.default );
  };
}
