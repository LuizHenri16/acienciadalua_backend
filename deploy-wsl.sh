#!/bin/bash
set -e

echo "Autenticando no GHCR..."
echo $CR_PAT | docker login ghcr.io -u LuizHenri16 --password-stdin

echo "Baixando imagem mais recente..."
docker pull ghcr.io/luizhenri16/auth_api:latest

echo "Recriando containers..."
docker compose down
docker compose up -d

echo "✅ Deploy concluído!"
