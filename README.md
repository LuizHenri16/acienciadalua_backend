# A Ciência da Lua - API
A complete API for A Ciência da Lua website using NestJS, PostgreSQL and JWT.

## Endpoints

### Admin
- `POST /auth/admin/signup` - Register a new user
- `POST /auth/admin/signin` - Authenticate user and return tokens
- `POST /auth/refresh` - Generate a new access_token using a refresh_token

### Customer
- `POST /auth/user/signin` - Generate magic link for user authentication
- `GET /auth/verify-link` - Verify magic link and return tokens

## Running the app
```bash
npm run start:dev
```

- The app will be available at `http://localhost:3000`.
- The documentation will be available at `http://localhost:3000/api`.

## Implemented 
- Framework: NestJS
- Database: PostgreSQL
- ORM: Prisma
- JWT (Json Web Token)
- Password hashing using bcrypt and comparing using bcrypt
- Token refresh using jwt
- Swagger documentation
- Rate limiting using @nestjs/throttler
- Cron job for cleaning up expired or used tokens daily at midnight using @nestjs/schedule
- Mail service using Resend for sending magic links to customers
- Validation pipes - using class-validator 


## Status
- in-progress

## API em Go (produção)

A pasta [`api/src`](api/src) contém o port completo da API para Go (chi + sqlc +
pgx + golang-migrate), com os mesmos endpoints, contratos HTTP e variáveis de
ambiente. O `Dockerfile` da raiz — usado pela action de deploy — constrói essa
API Go para o serviço `backend_prod` da VPS. A implementação NestJS anterior
continua em `Dockerfile.node` para desenvolvimento local e rollback.

```bash
cd api/src
cp .env.example .env
go run ./cmd/api                      # http://localhost:3000 (Swagger em /api)
docker compose --profile go up -d api-go   # na raiz, expõe http://localhost:3001
```

Detalhes de arquitetura, migrations sobre um banco do Prisma, deploy na VPS,
testes e diferenças conhecidas: [`api/src/README.md`](api/src/README.md).

## Developers 
Luiz Henrique Bastos Santana