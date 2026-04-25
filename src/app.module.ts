import { Module } from '@nestjs/common';
import { AuthModule } from './auth/auth.module';
import { PrismaModule } from './prisma/prisma.module';
import { JwtModule } from './jwt/jwt.module';

@Module({
  imports: [PrismaModule, AuthModule, JwtModule],
  controllers: [],
  providers: [],
})
export class AppModule {}
