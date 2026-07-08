import { JwtService } from '@nestjs/jwt';
import { Injectable, UnauthorizedException, NotFoundException, BadRequestException } from '@nestjs/common';
import { CustomerSigninDTO, CustomerSetPasswordDTO, ForgotPasswordDTO, ResetPasswordDTO, SignInDTO, SignUpDTO } from './dtos/auth';
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

    // ─── Admin ──────────────────────────────────────────────────────────────────

    private generateAccessToken(payload: { id: string; email: string; role: string }) {
        return this.jwtService.sign(payload, {
            secret: process.env.JWT_SECRET,
            expiresIn: (process.env.JWT_EXPIRES_IN ?? '1h') as never,
        });
    }

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

    // ─── Customer (password-based) ──────────────────────────────────────────────

    private generateCustomerToken(customer: { id: string; email: string; name: string }) {
        return this.jwtService.sign(
            { sub: customer.id, email: customer.email, name: customer.name },
            { secret: process.env.JWT_SECRET_CUSTOMER }
        );
    }

    async customerSignIn(data: CustomerSigninDTO) {
        const customer = await this.prismaSevice.customer.findUnique({
            where: { email: data.email }
        });

        if (!customer) {
            throw new UnauthorizedException("Customer not found");
        }

        if (!customer.password) {
            throw new UnauthorizedException("NO_PASSWORD_SET");
        }

        if (!(await this.passwordService.comparePassword(data.password, customer.password))) {
            throw new UnauthorizedException("Email or password wrong.");
        }

        return { access_token: this.generateCustomerToken(customer) };
    }

    async customerSetPassword(data: CustomerSetPasswordDTO) {
        const customer = await this.prismaSevice.customer.findUnique({
            where: { email: data.email }
        });

        if (!customer) {
            throw new NotFoundException("Customer not found");
        }

        if (customer.password) {
            throw new BadRequestException("Password already set. Use forgot password to reset.");
        }

        const hashedPassword = await this.passwordService.hashPassword(data.password);

        await this.prismaSevice.customer.update({
            where: { id: customer.id },
            data: { password: hashedPassword }
        });

        return { access_token: this.generateCustomerToken(customer) };
    }

    async customerForgotPassword(data: ForgotPasswordDTO) {
        const customer = await this.prismaSevice.customer.findUnique({
            where: { email: data.email }
        });

        if (!customer) {
            // Don't reveal if email exists
            return;
        }

        const resetToken = crypto.randomUUID();
        const expiresAt = new Date(Date.now() + 60 * 60 * 1000); // 1h

        await this.prismaSevice.customer.update({
            where: { id: customer.id },
            data: {
                resetToken,
                resetTokenExpiresAt: expiresAt,
            }
        });

        await this.emailService.sendResetPasswordEmail(customer.email, resetToken);
    }

    async customerResetPassword(data: ResetPasswordDTO) {
        const customer = await this.prismaSevice.customer.findFirst({
            where: {
                resetToken: data.token,
                resetTokenExpiresAt: { gt: new Date() }
            }
        });

        if (!customer) {
            throw new BadRequestException("Invalid or expired reset token.");
        }

        const hashedPassword = await this.passwordService.hashPassword(data.password);

        await this.prismaSevice.customer.update({
            where: { id: customer.id },
            data: {
                password: hashedPassword,
                resetToken: null as any,
                resetTokenExpiresAt: null as any,
            }
        });

        return { access_token: this.generateCustomerToken(customer) };
    }

    // TODO: REMOVER ANTES DE IR PARA PRODUÇÃO
    async customerDevToken(email: string) {
        const customer = await this.prismaSevice.customer.findUnique({ where: { email } });
        if (!customer) throw new UnauthorizedException("Customer not found");

        return this.generateCustomerToken(customer);
    }
}
