{ lib, stdenv, buildGoModule, fetchFromGitHub, installShellFiles }:
buildGoModule {
  pname   = "oplogtoredis";
  version = "2.0.1";

  src = builtins.path { path = ./.; };

  vendorSha256 = null;
  postInstall  = ''
  '';
  nativeBuildInputs = [installShellFiles];
  doCheck           = false;
  doInstallCheck    = false;
}
