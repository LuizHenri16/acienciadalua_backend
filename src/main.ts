import { NestFactory } from '@nestjs/core';
import { NestExpressApplication } from '@nestjs/platform-express';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import { join } from 'path';

async function bootstrap() {
  const app = await NestFactory.create<NestExpressApplication>(AppModule);

  // CORS
  app.enableCors({
    origin: process.env.FRONTEND_URL ?? 'http://localhost:3001',
    methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
    allowedHeaders: ['Content-Type', 'Authorization'],
    credentials: true,
  });

  // global pipe to validate DTOs
  app.useGlobalPipes(new ValidationPipe({
    whitelist: true,           // remove fields not declared in the DTO
    forbidNonWhitelisted: true, // return an error if an unknown field is received
    transform: true,           // convert body to DTO class instance
  }));

  // Expose uploads folder to access the images
  app.useStaticAssets(join(process.cwd(), 'uploads'), {
    prefix: '/uploads',
  });

  // Swagger setup to access the documentation
  const config = new DocumentBuilder()
    .setTitle('A Ciência da Lua - API')
    .setDescription('API responsible for A Ciência da Lua.')
    .setVersion('0.0.2')
    .addBearerAuth() // enables the "Authorize" button for JWT in the Swagger UI
    .build();

  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api', app, document); // docs available at /api

  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
