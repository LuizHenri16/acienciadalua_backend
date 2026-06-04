import { Module } from '@nestjs/common';
import { PaymentService } from './payment.service';
import { PaymentController } from './payment.controller';
import { ProductsModule } from '../products/products.module';
import { PrismaModule } from '../prisma/prisma.module';
import { EmailModule } from '../email/email.module';

@Module({
  imports: [ProductsModule, PrismaModule, EmailModule],
  providers: [PaymentService],
  controllers: [PaymentController]
})
export class PaymentModule { }
