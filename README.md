# A Ciência da Lua — API

API em Go (chi + sqlc + pgx + golang-migrate) com PostgreSQL e JWT, usada pelo
site A Ciência da Lua.

## Stack

- **HTTP:** chi v5
- **Banco:** PostgreSQL 15
- **Acesso a dados:** sqlc + pgx v5
- **Migrations:** golang-migrate (embutidas no binário)
- **Auth:** JWT (`golang-jwt/jwt/v5`) + bcrypt
- **Docs:** OpenAPI em `/api-json` e Swagger UI em `/api`
- **Pagamentos:** Mercado Pago (PIX e checkout)
- **E-mail:** Resend

O código-fonte fica em [`api/src`](api/src). Detalhes de arquitetura,
variáveis de ambiente, endpoints, migrations, testes e deploy estão em
[`api/src/README.md`](api/src/README.md).

## Rodando localmente

Com Docker Compose (banco + API na porta 3001):

```bash
cp .env.example .env    # ajuste DB_*, JWT_* e credenciais
docker compose up -d --build
```

A API fica em `http://localhost:3001` (Swagger em `/api`, spec em `/api-json`,
healthcheck em `/health`).

Sem Docker:

```bash
cd api/src
cp .env.example .env
go run ./cmd/api        # http://localhost:3000
```

## Endpoints principais

### Admin
- `POST /auth/admin/signup` — cadastra um usuário
- `POST /auth/admin/signin` — autentica e devolve os tokens
- `POST /auth/admin/refresh` — renova o access token

### Cliente
- `POST /auth/user/signin` — envia magic link de acesso
- `POST /auth/user/forgot-password` / `POST /auth/user/reset-password`
- `GET /auth/verify-link` — valida o magic link e devolve os tokens

### Catálogo e pedidos
- `GET /products`, `GET /products/:id`
- `POST/PUT/PATCH/DELETE /products` (admin)
- `POST /payment/create-preferences`, `POST /payment/pix`
- `POST /payment/status`, `POST /payment/webhook`
- `GET /customers/me`, `GET /customers/me/purchases`
- `GET /customers/me/downloads/:productId`

Lista completa em [`api/src/README.md`](api/src/README.md#endpoints).

## Deploy

O push para `main` dispara `.github/workflows/deploy.yml`, que constrói o
`Dockerfile` da raiz, publica a imagem e atualiza o serviço `backend_prod` na
VPS. O serviço expõe a porta interna `3000` para o proxy reverso.

## Developers
Luiz Henrique Bastos Santana
