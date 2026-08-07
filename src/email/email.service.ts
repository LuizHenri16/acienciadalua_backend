import { Injectable } from '@nestjs/common';
import { Resend } from 'resend';

@Injectable()
export class EmailService {
  private resend = new Resend(process.env.RESEND_API_KEY);

  constructor() {}

  async sendWelcomeEmail(email: string, name: string) {
    const { error } = await this.resend.emails.send({
      from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
      to: email,
      subject: 'Bem-vindo ao A Ciência da Lua',
      html: `
                <h1>Olá, ${name}!</h1>
                <p>Seu pagamento foi confirmado. Acesse seus materiais pelo link abaixo:</p>
                <a href="${process.env.FRONTEND_URL}/minha-conta/signin">Acessar minha conta</a>
            `,
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
      subject: 'Redefinição de senha - A Ciência da Lua',
      html: `
                <h1>Redefinição de senha</h1>
                <p>Clique no link abaixo para redefinir sua senha:</p>
                <a href="${url}">Redefinir senha</a>
                <p>Este link expira em 1 hora.</p>
                <p>Se você não solicitou esta alteração, ignore este email.</p>
            `,
    });

    if (error) {
      console.error('[EmailService] Resend error (reset):', error);
      throw new Error(`Failed to send reset email: ${error.message}`);
    }

    console.log('[EmailService] Reset email sent. Email ID:', data?.id);
  }

  async sendContactEmail(
    name: string,
    email: string,
    subject: string,
    message: string,
  ) {
    const { data, error } = await this.resend.emails.send({
      from: process.env.EMAIL_SENDER_ADDRESS ?? 'onboarding@resend.dev',
      to: process.env.CONTACT_EMAIL ?? 'contato@acienciadalua.com.br',
      replyTo: email,
      subject: subject
        ? `[Contato Site] ${subject}`
        : `[Contato Site] Mensagem de ${name}`,
      html: `
                <div style="font-family: sans-serif; max-width: 600px; margin: 0 auto;">
                    <h2 style="color: #0b7a75;">Nova mensagem de contato</h2>
                    <table style="width: 100%; border-collapse: collapse;">
                        <tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Nome</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">${name}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Email</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">${email}</td>
                        </tr>
                        ${
                          subject
                            ? `<tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Assunto</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">${subject}</td>
                        </tr>`
                            : ''
                        }
                    </table>
                    <div style="margin-top: 16px; padding: 16px; background: #f8fafc; border-radius: 8px;">
                        <h3 style="color: #1e293b; margin: 0 0 8px;">Mensagem</h3>
                        <p style="color: #334155; line-height: 1.6; white-space: pre-wrap;">${message}</p>
                    </div>
                    <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 24px 0;" />
                    <p style="color: #94a3b8; font-size: 12px;">Enviado do formulário de contato do site A Ciência da Lua.</p>
                </div>
            `,
    });

    if (error) {
      console.error('[EmailService] Resend error (contact):', error);
      throw new Error(`Failed to send contact email: ${error.message}`);
    }

    console.log('[EmailService] Contact email sent. Email ID:', data?.id);
  }
}
