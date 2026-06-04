# Stage 1 — deps: install ALL dependencies (including devDeps for build)
FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

# Stage 2 — build: compile TypeScript and generate Prisma client
FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npx prisma generate
RUN npm run build

# Stage 3 — production: lean image with only what's needed to run
FROM node:22-alpine AS production
WORKDIR /app
ENV NODE_ENV=production

COPY package*.json ./
RUN npm ci --omit=dev

# Copia o build com a estrutura mantida (dist/src/main)
COPY --from=build /app/dist ./dist
COPY --from=build /app/prisma ./prisma
COPY --from=build /app/prisma.config.js ./prisma.config.js
COPY --from=build /app/node_modules/@prisma/client ./node_modules/@prisma/client
COPY --from=build /app/node_modules/.prisma ./node_modules/.prisma

COPY --from=build /app/node_modules/prisma ./node_modules/prisma
COPY --from=build /app/node_modules/.bin ./node_modules/.bin

EXPOSE 3000

# Executa as migrations e inicia a partir de dist/src/main (com o .js explicitado)
CMD ["sh", "-c", "npx prisma migrate deploy && node dist/src/main.js"]