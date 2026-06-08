#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

git pull origin main

docker compose up -d --build db redis

make migrate-up

docker compose up -d --build app

curl -fsS http://localhost:8080/healthz

curl -fsS http://localhost:8080/readyz

docker compose ps
