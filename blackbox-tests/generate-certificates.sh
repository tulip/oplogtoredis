#!/bin/bash
set -eu

openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem \
  -subj "/C=US/ST=MA/L=Boston/O=Tulip/OU=techops/CN=example.com"
openssl genrsa -out server-key.pem 4096
openssl req -subj "/CN=localhost" -sha256 -new -key server-key.pem -out server.csr
echo "subjectAltName = DNS:redis" > extfile.cnf
openssl x509 -req -days 365 -sha256 -in server.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out server-cert.pem -extfile extfile.cnf
echo "Finished generating certs"
# This is because we might be running this command as root.
chmod go+r server-key.pem # perms will be restricted later
