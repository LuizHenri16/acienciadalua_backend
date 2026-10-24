import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import 'dotenv/config';
import type { CustomerJwtPayload, CustomerRequest } from './jwt.types';

@Injectable()
export class JwtCustomerGuard implements CanActivate {
  constructor(private readonly jwtService: JwtService) {}

  canActivate(context: ExecutionContext): boolean {
    const request = context.switchToHttp().getRequest<CustomerRequest>();
    const authHeader = request.headers.authorization;

    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      throw new UnauthorizedException(
        'Missing or invalid authorization header.',
      );
    }

    const token = authHeader.split(' ')[1];

    try {
      const payload = this.jwtService.verify<CustomerJwtPayload>(token, {
        secret: process.env.JWT_SECRET_CUSTOMER,
      });
      request.customer = payload;
      return true;
    } catch {
      throw new UnauthorizedException('Invalid or expired token.');
    }
  }
}
