# A Ciência da Lua — API em Go

Port do backend NestJS (`src/`, Prisma) para Go, preservando os mesmos
contratos HTTP e as mesmas variáveis de ambiente. A especificação do porte está
no plano `PORTO_BACKEND_NESTJS_PARA_GO.md` (na raiz do workspace, fora deste
repositório).

## Stack

| Camada | NestJS | Go |
|---|---|---|
| HTTP | NestJS 11 + Express | chi v5 |
| ORM | Prisma 7 | sqlc + pgx v5 |
| Migrations | `prisma migrate` | golang-migrate (embutido no binário) |
| Validação | class-validator | go-playground/validator + DTOs com as mesmas mensagens |
| Auth | `@nestjs/jwt` + guards | `golang-jwt/jwt/v5` + middlewares |
| Hash | bcrypt (10 rounds) | `golang.org/x/crypto/bcrypt` (10 rounds) |
| Rate limit | `@nestjs/throttler` | sliding window em memória por rota |
| Agendamento | `@nestjs/schedule` | `robfig/cron` |
| Docs | Swagger em `/api` | OpenAPI em `/api-json` + Swagger UI em `/api` |
| Pagamentos | SDK `mercadopago` | cliente REST (`api.mercadopago.com`) |
| E-mail | `resend` | cliente REST (`api.resend.com`) |

## Estrutura

```
api/src/
├── cmd/api/main.go             # bootstrap, router, middlewares, static uploads
├── internal/
│   ├── config/                 # leitura/validação do ambiente
│   ├── database/               # pool pgx + migrations embutidas
│   ├── db/                     # código gerado pelo sqlc (não editar)
│   ├── dto/                    # request DTOs + mensagens de validação
│   ├── handlers/               # camada HTTP (controllers)
│   ├── middleware/             # auth, CORS, rate limit, logger, recoverer
│   ├── services/               # regras de negócio (auth, products, payment, ...)
│   ├── jobs/                   # cron de limpeza de tokens
│   ├── docs/                   # OpenAPI + Swagger UI
│   ├── types/                  # Decimal/Timestamp no formato do Prisma
│   └── utils/                  # erros, JSON, JWT, bcrypt, arquivos
├── migrations/                 # golang-migrate (embutido via //go:embed)
├── sql/queries/                # queries do sqlc
├── sql/schema.sql              # hint de tipos para o sqlc (não executado)
├── sqlc.yaml
├── stress/                     # teste de estresse (programa Go independente)
├── Dockerfile
└── go.mod
```

## Desenvolvimento

Requisitos: Go 1.23+ (o `go.mod` declara 1.23) e PostgreSQL.

```bash
cd api/src
cp .env.example .env        # ajuste DATABASE_URL/JWT_*
go run ./cmd/api            # sobe em http://localhost:3000
```

A API carrega `.env` automaticamente (via `godotenv`, sem sobrescrever
variáveis já presentes no ambiente — ideal para containers).

Comandos úteis:

```bash
make run                # go run ./cmd/api
make build              # binário em bin/api
make test               # testes unitários
make test-integration   # requer TEST_DATABASE_URL (trunca tabelas!)
make fmt vet            # gofmt + go vet
make sqlc               # regenera internal/db (sqlc v1.27+)
make docker-build       # docker build -t acienciadalua-api-go .
make stress STRESS_ARGS="-scenario=read -requests=5000"  # carga (API no ar)
```

## Variáveis de ambiente

Os nomes são **exatamente** os mesmos do backend NestJS, então um `.env` já
existente funciona sem alterações:

`DATABASE_URL`, `JWT_SECRET`, `JWT_EXPIRES_IN`, `JWT_REFRESH_SECRET`,
`JWT_REFRESH_EXPIRES_IN`, `JWT_SECRET_CUSTOMER`, `NODE_ENV`, `PORT`, `APP_URL`,
`RESEND_API_KEY`, `EMAIL_SENDER_ADDRESS`, `CONTACT_EMAIL`,
`MERCADO_PAGO_ACCESS_TOKEN`, `MP_WEBHOOK_URL`, `MP_WEBHOOK_SECRET`,
`MP_SUCCESS_URL`, `MP_FAILURE_URL`, `MP_PENDING_URL`.

Variáveis opcionais (só da versão Go):

| Variável | Padrão | Função |
|---|---|---|
| `FRONTEND_URL` | valor de `APP_URL` | base dos links nos e-mails |
| `CORS_ORIGINS` | lista fixa do NestJS | origens CORS separadas por vírgula |
| `RUN_MIGRATIONS` | `true` | roda as migrations embutidas ao iniciar |
| `UPLOAD_DIR` | `uploads` | diretório servido em `/uploads` |
| `FILES_DIR` | `files` | diretório dos arquivos para download |
| `PAYMENT_STATUS_SECRET` | `JWT_SECRET_CUSTOMER` | segredo do token de status do PIX |
| `PAYMENT_STATUS_TTL` | `24h` | validade do token de status |
| `MP_PIX_EXPIRATION` | `30m` | validade do QR Code PIX |
| `MP_STATUS_TIMEOUT` | `5s` | timeout da consulta de status ao MP |
| `PAYMENT_RECONCILE_ENABLED` | `true` | liga a reconciliação de pendentes |
| `PAYMENT_RECONCILE_INTERVAL` | `2m` | intervalo da reconciliação |

> `JWT_EXPIRES_IN`/`JWT_REFRESH_EXPIRES_IN` aceitam o formato do
> `jsonwebtoken` (`1h`, `7d`, `30m`) e também segundos (`3600`).
>
> Tokens de cliente expiram em 1h, como no NestJS (o `expiresIn: '1h'` do
> `JwtModule` também era aplicado ao `JWT_SECRET_CUSTOMER`).

## Migrations e adoção de um banco existente

As migrations são embutidas no binário e aplicadas no startup
(`RUN_MIGRATIONS=true`). O baseline (`migrations/000001_init.up.sql`) é
**idempotente** (`CREATE TABLE IF NOT EXISTS`, `DO $$ ... EXCEPTION WHEN
duplicate_object`, etc.), de modo que ele pode ser aplicado sobre um banco que
já foi gerenciado pelo Prisma:

- o schema resultante é o mesmo do `prisma/schema.prisma` (tipos `TEXT`,
  `TIMESTAMP(3)`, `DECIMAL(65,30)` e enum `"Category"`);
- o golang-migrate grava o controle em `schema_migrations`;
- a tabela `_prisma_migrations` é deixada intacta — é possível voltar para o
  NestJS sem rollback destrutivo.

Para regenerar os tipos após alterar `sql/queries/` ou `migrations/`:

```bash
sqlc generate
```

> `sql/schema.sql` existe apenas porque o sqlc não lê o corpo de blocos `DO $$`.
> Ele redeclara o enum `"Category"` para o gerador e **nunca é executado**.

## Endpoints

Idênticos ao NestJS (mesmos paths, bodies e formatos de resposta):

| Método | Rota | Auth |
|---|---|---|
| POST | `/auth/admin/signup` | — |
| POST | `/auth/admin/signin` | — |
| POST | `/auth/admin/refresh` | — |
| POST | `/auth/user/signin` | — |
| POST | `/auth/user/password` | — |
| POST | `/auth/user/forgot-password` | — |
| POST | `/auth/user/reset-password` | — |
| GET | `/products` | — |
| GET | `/products/:id` | — |
| POST | `/products` | admin |
| PUT | `/products/:id` | admin |
| PATCH | `/products/:id/toggle` | admin |
| DELETE | `/products/:id` | admin |
| POST | `/payment/create-preferences` | — |
| POST | `/payment/pix` | — |
| POST | `/payment/status` | token de status |
| POST | `/payment/webhook` | assinatura do MP (quando configurada) |
| POST | `/contact` | — |
| GET | `/customers/me` | customer |
| GET | `/customers/me/purchases` | customer |
| GET | `/customers/me/downloads/:productId` | customer |
| GET | `/uploads/*` | — (estático) |
| GET | `/api`, `/api-json` | — (Swagger) |
| GET | `/health` | — (extra, healthcheck) |

Rate limiting (equivalente ao `ThrottlerModule`): 10 req/60s nas rotas de auth
admin; 10 req/15min em `/auth/user/signin`; 3 req/15min em
`/auth/user/forgot-password`.

### Contratos mantidos

- `Decimal` serializado como **string** (`"49.9"`), como no Prisma.
- Datas em `YYYY-MM-DDTHH:mm:ss.mmmZ` (ISO-8601 UTC com milissegundos).
- Erros no formato NestJS: `{ "message", "error", "statusCode" }`, com
  `message` em array para falhas de validação.
- 404/405 com `Cannot <METHOD> <path>`.
- Uploads multipart (`cover` → `uploads/`, `file` → `files/`), mesmas extensões
  permitidas (`.jpg/.jpeg/.png` e `.pdf/.zip`).
- Webhook do Mercado Pago idempotente por `mercadoPagoPaymentId` (índice único +
  `ON CONFLICT DO NOTHING` dentro de transação).

### Confirmação de pagamento PIX

O Mercado Pago é a fonte da verdade. O frontend nunca declara um pagamento como
pago: ele envia o `paymentId` e o `status_token` devolvidos por `/payment/pix`
para `POST /payment/status`.

```json
POST /payment/status
{ "paymentId": "987654321", "token": "<status_token>" }
```

A resposta traz `status` normalizado (`pending`, `approved`, `rejected`,
`expired`, `refunded`, `error`) e `accessGranted`. `approved` só é retornado com
`accessGranted: true` quando existe uma `Purchase` correspondente; o endpoint
reconcilia a compra de forma idempotente quando o MP informa `approved`.

A tabela aditiva `PaymentIntent` guarda o ciclo de vida do PIX (criada pela
migration `000002_payment_intent`). O webhook, o endpoint de status e o job de
reconciliação (`PAYMENT_RECONCILE_INTERVAL`) convergem para o mesmo
`ensurePurchase`, portanto qualquer um deles cobre um webhook perdido. A
criação de PIX aceita `idempotencyKey` e repete o mesmo QR em retries.

## Docker

Build isolado:

```bash
cd api/src
docker build -t acienciadalua-api-go .
docker run --rm -p 3001:3000 --env-file .env acienciadalua-api-go
```

Via compose (na raiz do backend), em paralelo com a API Node:

```bash
docker compose --profile go up -d api-go   # http://localhost:3001
```

O serviço `api-go` usa o mesmo `db` do compose, monta `./uploads` e `./files`
e, no compose local, roda em paralelo ao serviço `api` (NestJS).

### Deploy na VPS (imagem de produção)

A imagem de produção é construída pelo `Dockerfile` da **raiz do repositório**
(contexto `.`, que copia `api/src/`). O workflow
`.github/workflows/deploy.yml`, em push para `main`, publica essa imagem como
`<DOCKERHUB_USERNAME>/acienciadalua-backend:latest` e executa na VPS:

```bash
cd /root/acienciadalua
docker compose pull backend_prod
docker compose up -d --no-deps backend_prod
```

O serviço `backend_prod` continua usando a mesma imagem/tag e a porta interna
`3000`, então o proxy reverso não precisa mudar. O compose da VPS fica assim
(ajuste o `DATABASE_URL` conforme o seu `.env`):

```yaml
backend_prod:
  image: luizhenri16/acienciadalua-backend:latest
  container_name: acienciadalua_backend_prod
  restart: always
  depends_on:
    - database_prod
  env_file:
    - .env
  environment:
    - DATABASE_URL=postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@database_prod:5432/${POSTGRES_DB}?schema=public
    - PORT=3000
    - RUN_MIGRATIONS=true
  # command: ...  # remover: a imagem Go roda as migrations no startup
  volumes:
    - ./uploads:/app/uploads
    - ./files:/app/files
  networks:
    - acienciadalua_network_prod
```

A imagem usa `ENTRYPOINT`, então mesmo que o `command` antigo
(`npx prisma migrate deploy && node dist/main.js`) permaneça, o binário Go sobe
e ignora os argumentos. Ainda assim, remova o `command` para não confundir.

Antes do primeiro deploy, confirme no `.env` da VPS:

- `JWT_SECRET`, `JWT_REFRESH_SECRET` e `JWT_SECRET_CUSTOMER` definidos;
- `DATABASE_URL` apontando para o banco atual;
- `APP_URL` e `MP_*` com as URLs públicas de produção;
- `RUN_MIGRATIONS=true` (padrão) para aplicar o baseline idempotente;
- `uploads/` e `files/` graváveis pelo uid `1000` (a imagem roda como `app`):
  `chown -R 1000:1000 uploads files`.

Rollback: antes do primeiro deploy Go, guarde a imagem Node atual na VPS:

```bash
docker tag luizhenri16/acienciadalua-backend:latest \
  luizhenri16/acienciadalua-backend:node-rollback
```

Se precisar voltar:

```bash
docker tag luizhenri16/acienciadalua-backend:node-rollback \
  luizhenri16/acienciadalua-backend:latest
docker compose up -d --no-deps backend_prod
```

O `Dockerfile.node` também preserva o build NestJS no repositório; um
`git revert` do commit de cutover + push para `main` reconstrói a imagem Node.

## Testes

```bash
go test ./...                       # unitários (sem banco)
TEST_DATABASE_URL='postgresql://test:test@localhost:5432/acienciadalua?schema=public' \
  go test ./internal/services -v    # integração: webhook idempotente + contratos MP
```

Os testes de integração truncam `Purchase`, `AuthToken`, `Product`, `Customer`
e `User` — use um banco descartável.

## Teste de estresse

`stress/` é um programa Go independente (só biblioteca padrão) que envia
requisições contra uma API em execução e reporta latência, throughput, códigos
HTTP e erros.

```bash
cd api/src
go run ./stress -scenario=read -concurrency=20 -requests=5000
go run ./stress -base-url=http://localhost:3000 -scenario=read -duration=30s
```

O padrão é a API Go em `http://localhost:3001`; use `-base-url` para apontar
para outra instância (por exemplo, a NestJS na 3000).

Cenários: `read` (padrão, sem escritas), `webhook`, `auth`, `crud` (ciclo de
produto, exige admin e limpa o que criou) e `mixed`. Use `-max-error-rate` para
falhar em CI e `-json` para saída legível por máquina. Flags, avisos de rate
limit e cuidados com escritas estão em [`stress/README.md`](stress/README.md).

## Diferenças conhecidas (menores)

Todas deliberadas, para manter compatibilidade ou robustez:

1. `GET /products?category=INVALID` responde **400** com a mensagem de enum.
   No NestJS/Prisma o valor inválido chegava ao banco e virava **500**.
2. Preços em JSON são aceitos apenas como **número** (como no
   `class-transformer`); strings são rejeitadas com 400.
3. O webhook também aceita notificações via query string
   (`?type=payment&data.id=...`), além do corpo JSON. O comportamento anterior
   (corpo) continua idêntico.
4. `GET /customers/me` mantém `password` e `resetToken` na resposta, pois era
   exatamente o que o Prisma retornava (`findUnique` sem `select`).
5. `GET /health` é um endpoint adicional (não existe no NestJS) usado pelo
   healthcheck do container.
6. `/api-yaml` não é servido; apenas `/api` (UI) e `/api-json` (spec).
