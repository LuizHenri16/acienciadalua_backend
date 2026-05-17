import { Injectable } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import { Resend } from 'resend';

@Injectable()
export class EmailService {

    private resend = new Resend(process.env.RESEND_API_KEY);

    constructor(
        private prismaService: PrismaService
    ) { }

    async generateMagicLink(email: string, customerId: string) {
        const token = crypto.randomUUID();
        const expiryDate = new Date(Date.now() + 15 * 60 * 1000);

        await this.prismaService.authToken.create({
            data: {
                token,
                expiresAt: expiryDate,
                customerId: customerId,
                usedAt: null as unknown as Date,
            },
        });

        await this.sendMagicLink(email, token);
    }

    async sendMagicLink(email: string, token: string) {
        const url = `${process.env.MAGIC_LINK_URL}${token}`;

        try {
            await this.resend.emails.send({
                from: `${process.env.EMAIL_SENDER_ADDRESS}`,
                to: email,
                subject: "Login em A Ciência da Lua",
                html: `<a href="${url}">Entre em A Ciência da Lua</a>`,
            });
        } catch (error) {
            throw new Error("Failed to send magic link");
        }
    }

    async sendWelcomeEmail(email: string, name: string) {
        try {
            await this.resend.emails.send({
                from: `${process.env.EMAIL_SENDER_ADDRESS}`,
                to: email,
                subject: "Bem-vindo ao A Ciência da Lua",
                html: `
                <h1>Olá, ${name}!</h1>
                <p>Seu pagamento foi confirmado. Acesse seus materiais pelo link abaixo:</p>
                <a href="${process.env.FRONTEND_URL}/minha-conta">Acessar minha conta</a>
            `
            });
        } catch (error) {
            throw new Error("Failed to send welcome email");
        }
    }
}
