import { Module } from '@nestjs/common';
import { MulterModule } from '@nestjs/platform-express';
import { ProductsService } from './products.service';
import { ProductsController } from './products.controller';
import { PrismaModule } from '../prisma/prisma.module';
import { JwtModule } from '../jwt/jwt.module';

@Module({
  imports: [
    PrismaModule,
    MulterModule.register({ dest: './uploads' }),
    JwtModule
  ],
  providers: [ProductsService],
  controllers: [ProductsController],
})
export class ProductsModule { }
