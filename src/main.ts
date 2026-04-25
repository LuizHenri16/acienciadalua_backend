import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  // global pipe to validate DTOs
  app.useGlobalPipes(new ValidationPipe({
    whitelist: true,           // remove fields not declared in the DTO
    forbidNonWhitelisted: true, // return an error if an unknown field is received
    transform: true,           // convert body to DTO class instance
  }));

  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
