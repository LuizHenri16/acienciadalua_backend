import { Module } from '@nestjs/common';
import { ScheduleModule } from '@nestjs/schedule';
import { ThrottlerModule } from '@nestjs/throttler';
import { AuthModule } from './auth/auth.module';
import { PrismaModule } from './prisma/prisma.module';
import { JwtModule } from './jwt/jwt.module';
import { EmailModule } from './email/email.module';
import { ContactModule } from './contact/contact.module';
import { ProductsModule } from './products/products.module';
import { PaymentModule } from './payment/payment.module';
import { CustomersModule } from './customers/customers.module';

@Module({
  imports: [
    ThrottlerModule.forRoot([
      {
        ttl: 60000,
        limit: 10,
      },
    ]),
    ScheduleModule.forRoot(),
    PrismaModule,
    AuthModule,
    JwtModule,
    EmailModule,
    ContactModule,
    ProductsModule,
    PaymentModule,
    CustomersModule,
  ],
  controllers: [],
  providers: [],
})
export class AppModule {}
