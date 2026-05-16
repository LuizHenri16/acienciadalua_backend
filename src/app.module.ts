import { Module } from '@nestjs/common';
import { ScheduleModule } from '@nestjs/schedule';
import { ThrottlerModule } from '@nestjs/throttler';
import { AuthModule } from './auth/auth.module';
import { PrismaModule } from './prisma/prisma.module';
import { JwtModule } from './jwt/jwt.module';
import { EmailService } from './email/email.service';
import { EmailModule } from './email/email.module';
import { ProductsModule } from './products/products.module';

@Module({
  imports: [
    ThrottlerModule.forRoot([{
      ttl: 60000,
      limit: 10,
    }]),
    ScheduleModule.forRoot(),
    PrismaModule,
    AuthModule,
    JwtModule,
    EmailModule,
    ProductsModule,
  ],
  controllers: [],
  providers: [EmailService],
})
export class AppModule {}
