# Stage 1 — deps: install ALL dependencies (including devDeps for build)
FROM node:22-alpine AS deps

WORKDIR /app

COPY package*.json ./

RUN npm ci

# Stage 2 — build: compile TypeScript and generate Prisma client
FROM node:22-alpine AS build

WORKDIR /app

# Copy dependencies from previous stage
COPY --from=deps /app/node_modules ./node_modules

# Copy the source code
COPY . .

# Generate Prisma client
RUN npx prisma generate

# Compile TypeScript → dist/
RUN npm run build

# Stage 3 — production: lean image with only what's needed to run
FROM node:22-alpine AS production

WORKDIR /app

ENV NODE_ENV=production

# Install only production dependencies (no devDeps)
COPY package*.json ./
RUN npm ci --omit=dev

# Copy compiled output from build stage
COPY --from=build /app/dist ./dist

# Copy Prisma schema and generated client (needed at runtime)
COPY --from=build /app/node_modules/.prisma ./node_modules/.prisma
COPY prisma ./prisma

# Expose the application port
EXPOSE 3000

# Start the compiled application
CMD ["node", "dist/src/main"]
