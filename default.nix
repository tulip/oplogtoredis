{ lib, stdenv, buildGoModule, fetchFromGitHub, installShellFiles }:

buildGoModule {
  pname = "oplogtoredis";
  version = "3.7.2";
  src = builtins.path { path = ./.; };

  postInstall = ''
  '';

  # update: set value to an empty string and run `nix build`. This will download Go, fetch the dependencies and calculates their hash.
  vendorHash = "sha256-c8HNPvFY3hu4iPrQ1Z4IoWzt09gJGDA5ZX5a87vsW50=";

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
