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

        const { data, error } = await this.resend.emails.send({
            from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
            to: email,
            subject: "Login em A Ciência da Lua",
            html: `<a href="${url}">Entre em A Ciência da Lua</a>`,
        });

        if (error) {
            console.error('[EmailService] Resend error:', error);
            throw new Error(`Failed to send magic link: ${error.message}`);
        }

        console.log('[EmailService] Magic link sent. Email ID:', data?.id);
    }

    async sendWelcomeEmail(email: string, name: string) {
        const { error } = await this.resend.emails.send({
            from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
            to: email,
            subject: "Bem-vindo ao A Ciência da Lua",
            html: `
                <h1>Olá, ${name}!</h1>
                <p>Seu pagamento foi confirmado. Acesse seus materiais pelo link abaixo:</p>
                <a href="${process.env.FRONTEND_URL}/minha-conta">Acessar minha conta</a>
            `
        });

        if (error) {
            console.error('[EmailService] Resend error (welcome):', error);
            throw new Error(`Failed to send welcome email: ${error.message}`);
        }
    }
}
