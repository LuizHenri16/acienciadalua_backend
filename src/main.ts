import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  // global pipe to validate DTOs
  app.useGlobalPipes(new ValidationPipe({
    whitelist: true,           // remove fields not declared in the DTO
    forbidNonWhitelisted: true, // return an error if an unknown field is received
    transform: true,           // convert body to DTO class instance
  }));

  // Swagger setup to access the documentation
  const config = new DocumentBuilder()
    .setTitle('Auth API')
    .setDescription('API responsible for user authentication: signup, signin and token refresh.')
    .setVersion('1.0')
    .addBearerAuth() // enables the "Authorize" button for JWT in the Swagger UI
    .build();

  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api', app, document); // docs available at /api

  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
