import { Module } from '@nestjs/common';
import { AuthController } from './auth.controller';
import { AuthService } from './auth.service';
import { PrismaModule } from '../prisma/prisma.module';
import { JwtModule } from '../jwt/jwt.module';
import { PasswordService } from './password.service';
import { EmailService } from '../email/email.service';
import { AuthTokenCleanupService } from './auth-token-cleanup.service';

@Module({
  imports: [PrismaModule, JwtModule],
  controllers: [AuthController],
  providers: [AuthService, PasswordService, EmailService, AuthTokenCleanupService],
})
export class AuthModule { }
