import type { Request } from 'express';

/**
 * Payload assinado para o admin (segredo: JWT_SECRET).
 */
export interface AdminJwtPayload {
  id: string;
  email: string;
  role: string;
  iat?: number;
  exp?: number;
}

/**
 * Payload assinado para o cliente (segredo: JWT_SECRET_CUSTOMER).
 */
export interface CustomerJwtPayload {
  sub: string;
  email: string;
  name: string;
  iat?: number;
  exp?: number;
}

/**
 * Request com o admin autenticado — preenchido pelo JwtAdminGuard.
 */
export interface AdminRequest extends Request {
  user: AdminJwtPayload;
}

/**
 * Request com o cliente autenticado — preenchido pelo JwtCustomerGuard.
 */
export interface CustomerRequest extends Request {
  customer: CustomerJwtPayload;
}
