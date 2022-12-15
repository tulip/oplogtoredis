{ lib, stdenv, buildGoModule, fetchFromGitHub, installShellFiles }:
buildGoModule {
  pname   = "oplogtoredis";
  version = "2.0.1";

  src = builtins.path { path = ./.; };

  vendorSha256 = "sha256-VHiYVJUNtHN2IY4iXZ6kHAa3Avi2VwRH1ySKBrrCDu4=";
  postInstall  = ''
  '';
  nativeBuildInputs = [installShellFiles];
  doCheck           = false;
  doInstallCheck    = false;
}
