{ lib, stdenv, buildGoModule, fetchFromGitHub, installShellFiles }:

buildGoModule {
  pname = "oplogtoredis";
  version = "3.5.1";
  src = builtins.path { path = ./.; };

  postInstall = ''
  '';

  # update: set value to an empty string and run `nix build`. This will download Go, fetch the dependencies and calculates their hash.
  vendorHash = "sha256-S7/phL8nEYNVeDPqGjh3OAqVB8nOmYk0XDhD7op3fa4=";

  nativeBuildInputs = [ installShellFiles ];
  doCheck = false;
  doInstallCheck = false;

  meta = with lib; {
    description = ''
    This program tails the oplog of a Mongo server, and publishes changes to Redis.
    It's designed to work with the redis-oplog Meteor package'';
    homepage = "https://github.com/tulip/oplogtoredis";
    license = licenses.mit;
  };
}
