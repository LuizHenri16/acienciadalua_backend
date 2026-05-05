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

## Developers 
Luiz Henrique Bastos Santana