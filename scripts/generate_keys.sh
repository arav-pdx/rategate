#!/usr/bin/env bash
set -euo pipefail

mkdir -p certs
openssl genrsa -out certs/jwt_private.pem 2048
openssl rsa -in certs/jwt_private.pem -pubout -out certs/jwt_public.pem
echo "Generated certs/jwt_private.pem and certs/jwt_public.pem"
