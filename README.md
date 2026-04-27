# Auth API

A simple API for user authentication using NestJS, PostgreSQL and JWT.

## Endpoints

- `POST /auth/signup` - Register a new user
- `POST /auth/signin` - Authenticate user and return tokens
- `POST /auth/refresh` - Generate a new access_token using a refresh_token

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
- Validation pipes - using class-validator 