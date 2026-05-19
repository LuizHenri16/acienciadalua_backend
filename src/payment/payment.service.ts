import { Injectable, InternalServerErrorException } from '@nestjs/common';
import MercadoPagoConfig, { Payment, Preference } from 'mercadopago';
import { ProductsService } from '../products/products.service';
import { PrismaService } from '../prisma/prisma.service';
import { EmailService } from '../email/email.service';

@Injectable()
export class PaymentService {
    private client: MercadoPagoConfig;

    constructor(
        private readonly productsService: ProductsService,
        private readonly prismaService: PrismaService,
        private readonly emailService: EmailService
    ) {
        this.client = new MercadoPagoConfig({ accessToken: process.env.MERCADO_PAGO_ACCESS_TOKEN! });
    }

    async createPreferences(productId: string) {
        const product = await this.productsService.findOne(productId);
        const preference = new Preference(this.client)

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

                    },
                    auto_return: 'approved',
                }
            });
            return {
                init_point: response.init_point,
            };
        } catch (error) {
            console.log(error);
            throw new InternalServerErrorException(error.message);
        }
    }

    async handleWebhook(body: any) {
        if (body.type !== 'payment') { return; }

        const paymentId = body.data.id;
        const payment = await new Payment(this.client).get({ id: paymentId });

        if (payment.status !== 'approved') { return; }

        const alreadyProcessed = await this.prismaService.purchase.findUnique({
            where: { mercadoPagoPaymentId: String(paymentId) }
        })

        if (alreadyProcessed) { return; }

        // Se não tiver e-mail, não será processado o pedido
        if (!payment.payer?.email) { throw new InternalServerErrorException("Pagamento sem e-mail"); }

        // Buscar ou criar o cliente no banco de dados (upsert verifica se o cliente existe, se sim, atualiza, se não, cria)
        const customer = await this.prismaService.customer.upsert({
            where: { email: payment.payer.email },
            update: {},
            create: {
                email: payment.payer.email,
                name: payment.payer?.first_name ?? payment.payer.email,
            }
        })

        // Inserir a compra no banco de dados
        await this.prismaService.purchase.create({
            data: {
                customerId: customer.id,
                productId: payment.external_reference ?? '',
                mercadoPagoPaymentId: String(paymentId),
                amountPaid: Number(payment.transaction_amount),
                purchasedAt: new Date(payment.date_approved ?? payment.date_created ?? ''),
            }
        })

        await this.emailService.sendWelcomeEmail(customer.email, customer.name);
    }
}
