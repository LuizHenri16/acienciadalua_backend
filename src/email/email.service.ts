import { Injectable } from '@nestjs/common';
import { Resend } from 'resend';

@Injectable()
export class EmailService {

    private resend = new Resend(process.env.RESEND_API_KEY);

    constructor() { }

    async sendWelcomeEmail(email: string, name: string) {
        const { error } = await this.resend.emails.send({
            from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
            to: email,
            subject: "Bem-vindo ao A Ciência da Lua",
            html: `
                <h1>Olá, ${name}!</h1>
                <p>Seu pagamento foi confirmado. Acesse seus materiais pelo link abaixo:</p>
                <a href="${process.env.FRONTEND_URL}/minha-conta/signin">Acessar minha conta</a>
            `
        });

        if (error) {
            console.error('[EmailService] Resend error (welcome):', error);
            throw new Error(`Failed to send welcome email: ${error.message}`);
        }
    }

    async sendResetPasswordEmail(email: string, token: string) {
        const url = `${process.env.FRONTEND_URL}/minha-conta/redefinir-senha?token=${token}`;

        const { data, error } = await this.resend.emails.send({
            from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
            to: email,
            subject: "Redefinição de senha - A Ciência da Lua",
            html: `
                <h1>Redefinição de senha</h1>
                <p>Clique no link abaixo para redefinir sua senha:</p>
                <a href="${url}">Redefinir senha</a>
                <p>Este link expira em 1 hora.</p>
                <p>Se você não solicitou esta alteração, ignore este email.</p>
            `
        });

        if (error) {
            console.error('[EmailService] Resend error (reset):', error);
            throw new Error(`Failed to send reset email: ${error.message}`);
        }

        console.log('[EmailService] Reset email sent. Email ID:', data?.id);
    }
}
