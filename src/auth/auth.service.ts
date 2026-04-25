import { JwtService } from '@nestjs/jwt';
import { Injectable, UnauthorizedException } from '@nestjs/common';
import { SignInDTO, SignUpDTO } from './dtos/auth';
import { PrismaService } from '../prisma/prisma.service';
import { PasswordService } from './password.service';
import 'dotenv/config';

@Injectable()
export class AuthService {
    constructor(
        private prismaSevice: PrismaService,
        private jwtService: JwtService,
        private passwordService: PasswordService
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

        // Generated hash password using bcrypt
        const hashedPassword = await this.passwordService.hashPassword(data.password);
        data.password = hashedPassword;

        const user = await this.prismaSevice.user.create({ data });

        // After login generates access_token nd refresh_token
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
            // Validates the token using the refresh secret (throws if invalid or expired)
            const payload = this.jwtService.verify(refresh_token, {
                secret: process.env.JWT_REFRESH_SECRET,
            });

            // Generates a new access token with the same user data
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
}
