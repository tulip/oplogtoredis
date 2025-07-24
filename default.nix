{ lib, stdenv, buildGo124Module, fetchFromGitHub, installShellFiles }:

buildGo124Module {
  pname = "oplogtoredis";
  version = "3.9.0";
  src = builtins.path { path = ./.; };

  postInstall = ''
  '';

  # update: set value to an empty string and run `nix build`. This will download Go, fetch the dependencies and calculates their hash.
  vendorHash = "sha256-lVeF4HQPI/XqVoejdR7bk/DP66DIp56TVE4fW5F3U6A=";

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
