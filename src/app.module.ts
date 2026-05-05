import { Module } from '@nestjs/common';
import { AuthModule } from './auth/auth.module';
import { PrismaModule } from './prisma/prisma.module';
import { JwtModule } from './jwt/jwt.module';
import { EmailService } from './email/email.service';
import { EmailModule } from './email/email.module';

@Module({
  imports: [PrismaModule, AuthModule, JwtModule, EmailModule],
  controllers: [],
  providers: [EmailService],
})
export class AppModule {}
