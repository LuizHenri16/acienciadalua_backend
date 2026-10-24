/**
 * Tipos do Mercado Pago usados pela aplicação.
 *
 * O SDK oficial (`mercadopago`) tipa bem a maior parte da resposta, mas o
 * campo `metadata` e o corpo do webhook chegam como `any`. Aqui eles ficam
 * explícitos para que o resto do código trabalhe com tipos seguros.
 */
export interface MercadoPagoPaymentMetadata {
  customer_email?: string;
  customer_name?: string;
}

export interface MercadoPagoPayment {
  id?: number;
  status?: string;
  external_reference?: string;
  transaction_amount?: number;
  date_approved?: string;
  date_created?: string;
  payer?: {
    email?: string;
    first_name?: string;
  };
  metadata?: MercadoPagoPaymentMetadata;
  point_of_interaction?: {
    transaction_data?: {
      qr_code?: string;
      qr_code_base64?: string;
      ticket_url?: string;
    };
  };
}

/** Corpo enviado pelo Mercado Pago no webhook de notificações. */
export interface MercadoPagoWebhookBody {
  type?: string;
  action?: string;
  data?: {
    id?: string | number;
  };
}
