import { JwtService } from '@nestjs/jwt';
import { Injectable, UnauthorizedException } from '@nestjs/common';
import { CustomerSigninDTO, SignInDTO, SignUpDTO } from './dtos/auth';
import { PrismaService } from '../prisma/prisma.service';
import { PasswordService } from './password.service';
import 'dotenv/config';
import { EmailService } from '../email/email.service';

@Injectable()
export class AuthService {
    constructor(
        private prismaSevice: PrismaService,
        private jwtService: JwtService,
        private passwordService: PasswordService,
        private emailService: EmailService
    ) { }

    // Generates a short-lived access token (1h)
    private generateAccessToken(payload: { id: string; email: string; role: string }) {
        return this.jwtService.sign(payload, {
            secret: process.env.JWT_SECRET,
            expiresIn: (process.env.JWT_EXPIRES_IN ?? '1h') as never,
        });
    }

    // Generates a long-lived refresh token (7d) with a different secret
    private generateRefreshToken(payload: { id: string; email: string; role: string }) {
        return this.jwtService.sign(payload, {
            secret: process.env.JWT_REFRESH_SECRET,
            expiresIn: (process.env.JWT_REFRESH_EXPIRES_IN ?? '7d') as never,
        });
    }

    async signup(data: SignUpDTO) {
        const userAlreadyExists = await this.prismaSevice.user.findUnique({
            where: { email: data.email }
        });

        if (userAlreadyExists) {
            throw new UnauthorizedException("User already exists");
        }

        const hashedPassword = await this.passwordService.hashPassword(data.password);
        data.password = hashedPassword;

        const user = await this.prismaSevice.user.create({ data });

        const payload = { id: user.id, email: user.email, role: user.role };

        return {
            access_token: this.generateAccessToken(payload),
            refresh_token: this.generateRefreshToken(payload),
        };
    }

    async signin(data: SignInDTO) {
        const userExists = await this.prismaSevice.user.findUnique({
            where: { email: data.email }
        });

        if (!userExists || !(await this.passwordService.comparePassword(data.password, userExists.password))) {
            throw new UnauthorizedException("User email or password wrong.");
        }

        const payload = { id: userExists.id, email: userExists.email, role: userExists.role };

        return {
            access_token: this.generateAccessToken(payload),
            refresh_token: this.generateRefreshToken(payload),
        };
    }

    async refresh(refresh_token: string) {
        try {
            const payload = this.jwtService.verify(refresh_token, {
                secret: process.env.JWT_REFRESH_SECRET,
            });

            const newAccessToken = this.generateAccessToken({
                id: payload.id,
                email: payload.email,
                role: payload.role,
            });

            return { access_token: newAccessToken };

        } catch {
            throw new UnauthorizedException("Invalid or expired refresh token.");
        }
    }

    async customerGenerateMagicLink(data: CustomerSigninDTO) {
        const customer = await this.prismaSevice.customer.findUnique({
            where: { email: data.email }
        });

        if (!customer) {
            throw new UnauthorizedException("Customer not found");
        }

        await this.emailService.generateMagicLink(customer.email, customer.id);
    }

    // TODO: REMOVER ANTES DE IR PARA PRODUÇÃO
    async customerDevToken(email: string) {
        const customer = await this.prismaSevice.customer.findUnique({ where: { email } });
        if (!customer) throw new UnauthorizedException("Customer not found");

        return this.jwtService.sign(
            { sub: customer.id, email: customer.email, name: customer.name },
            { secret: process.env.JWT_SECRET_CUSTOMER }
        );
    }

    async customerVerifyMagicLink(token: string) {
        const tokenExists = await this.prismaSevice.authToken.findUnique({
            where: { token },
            include: {
                customer: true
            }
        });

        if (!tokenExists) throw new UnauthorizedException("Invalid magic link.");
        if (tokenExists.expiresAt < new Date()) throw new UnauthorizedException("Magic link expired.");
        if (tokenExists.usedAt) throw new UnauthorizedException("Used magic link.");

        await this.prismaSevice.authToken.update({
            where: { id: tokenExists.id },
            data: { usedAt: new Date() }
        });

        return this.jwtService.sign(
            { sub: tokenExists.customerId, email: tokenExists.customer.email, name: tokenExists.customer.name },
            { secret: process.env.JWT_SECRET_CUSTOMER }
        );
    }
}
