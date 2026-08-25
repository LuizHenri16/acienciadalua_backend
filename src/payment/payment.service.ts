import { Injectable, InternalServerErrorException } from '@nestjs/common';
import MercadoPagoConfig, { Payment, Preference } from 'mercadopago';
import { ProductsService } from '../products/products.service';
import { PrismaService } from '../prisma/prisma.service';
import { EmailService } from '../email/email.service';
import { ProcessPaymentDto } from './dtos/process-payment.dto';
import { CreatePixPaymentDto } from './dtos/create-pix-payment.dto';

@Injectable()
export class PaymentService {
  private client: MercadoPagoConfig;

  constructor(
    private readonly productsService: ProductsService,
    private readonly prismaService: PrismaService,
    private readonly emailService: EmailService,
  ) {
    this.client = new MercadoPagoConfig({
      accessToken: process.env.MERCADO_PAGO_ACCESS_TOKEN!,
    });
  }

  async createPreferences(productId: string) {
    const product = await this.productsService.findOne(productId);
    const preference = new Preference(this.client);

    try {
      const response = await preference.create({
        body: {
          items: [
            {
              id: product.id,
              title: product.title,
              quantity: 1,
              unit_price: Number(product.price),
            },
          ],
          notification_url: process.env.MP_WEBHOOK_URL,
          external_reference: product.id,
          back_urls: {
            success: process.env.MP_SUCCESS_URL,
            failure: process.env.MP_FAILURE_URL,
            pending: process.env.MP_PENDING_URL,
          },
          payment_methods: {
            excluded_payment_types: [{ id: 'ticket' }, { id: 'atm' }],
            excluded_payment_methods: [{ id: 'pec' }],
            installments: 6,
          },
          auto_return: 'approved',
        },
      });
      return {
        init_point: response.init_point,
        preference_id: response.id,
      };
    } catch (error) {
      console.log(error);
      throw new InternalServerErrorException(error.message);
    }
  }

  async processPayment(dto: ProcessPaymentDto) {
    const product = await this.productsService.findOne(dto.productId);

    const paymentClient = new Payment(this.client);

    try {
      const result = await paymentClient.create({
        body: {
          transaction_amount: Number(product.price),
          token: dto.token,
          description: product.title,
          installments: dto.installments,
          payment_method_id: dto.payment_method_id,
          issuer_id: Number(dto.issuer_id),
          payer: {
            email: dto.payer.email,
            ...(dto.payer.name && { first_name: dto.payer.name }),
            ...(dto.payer.identification && {
              identification: dto.payer.identification,
            }),
          },
          external_reference: product.id,
          notification_url: process.env.MP_WEBHOOK_URL,
        },
      });

      if (result.status === 'approved') {
        await this.registerApprovedPayment(result);
      }

      return {
        id: result.id,
        status: result.status,
        status_detail: result.status_detail,
      };
    } catch (error) {
      console.error('Erro ao processar pagamento:', error);
      throw new InternalServerErrorException(
        error.message || 'Erro ao processar pagamento',
      );
    }
  }

  async createPixPayment(dto: CreatePixPaymentDto) {
    const product = await this.productsService.findOne(dto.productId);

    const paymentClient = new Payment(this.client);

    try {
      const result = await paymentClient.create({
        body: {
          transaction_amount: Number(product.price),
          description: product.title,
          payment_method_id: 'pix',
          payer: {
            email: dto.payer.email,
            first_name: dto.payer.name,
            identification: dto.payer.cpf
              ? { type: 'CPF', number: dto.payer.cpf }
              : undefined,
          },
          external_reference: product.id,
          notification_url: process.env.MP_WEBHOOK_URL,
        },
      });

      const pixData = (result as any).point_of_interaction?.transaction_data;

      return {
        id: result.id,
        status: result.status,
        qr_code: pixData?.qr_code,
        qr_code_base64: pixData?.qr_code_base64,
        ticket_url: pixData?.ticket_url,
      };
    } catch (error) {
      console.error('Erro ao criar pagamento PIX:', error);
      throw new InternalServerErrorException(
        error.message || 'Erro ao criar pagamento PIX',
      );
    }
  }

  async handleWebhook(body: any) {
    if (body.type !== 'payment') {
      return;
    }

    const paymentId = body.data.id;
    const payment = await new Payment(this.client).get({ id: paymentId });

    if (payment.status !== 'approved') {
      return;
    }

    const alreadyProcessed = await this.prismaService.purchase.findUnique({
      where: { mercadoPagoPaymentId: String(paymentId) },
    });

    if (alreadyProcessed) {
      return;
    }

    await this.registerApprovedPayment(payment);
  }

  private async registerApprovedPayment(payment: any) {
    const paymentId = payment.id;

    const alreadyProcessed = await this.prismaService.purchase.findUnique({
      where: { mercadoPagoPaymentId: String(paymentId) },
    });

    if (alreadyProcessed) {
      return;
    }

    if (!payment.payer?.email) {
      throw new InternalServerErrorException('Pagamento sem e-mail');
    }

    const customer = await this.prismaService.customer.upsert({
      where: { email: payment.payer.email },
      update: {},
      create: {
        email: payment.payer.email,
        name: payment.payer?.first_name ?? payment.payer.email,
      },
    });

    await this.prismaService.purchase.create({
      data: {
        customerId: customer.id,
        productId: payment.external_reference ?? '',
        mercadoPagoPaymentId: String(paymentId),
        amountPaid: Number(payment.transaction_amount),
        purchasedAt: new Date(
          payment.date_approved ?? payment.date_created ?? Date.now(),
        ),
      },
    });

    await this.emailService.sendWelcomeEmail(customer.email, customer.name).catch((err) => {
      console.error('[EmailService] Erro ao enviar e-mail de boas-vindas:', err.message);
    });
  }
}
