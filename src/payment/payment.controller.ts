import { Body, Controller, HttpCode, HttpStatus, Post } from '@nestjs/common';
import { PaymentService } from './payment.service';
import { CreatePixPaymentDto } from './dtos/create-pix-payment.dto';
import {
  ApiOperation,
  ApiResponse,
  ApiTags,
} from '@nestjs/swagger';

@ApiTags('Payment')
@Controller('payment')
export class PaymentController {
  constructor(private readonly paymentService: PaymentService) {}

  @ApiOperation({
    summary: 'Cria uma preferência para pagamento',
    description: 'Cria uma preferência para pagamento com o ID do produto',
  })
  @ApiResponse({
    status: 201,
    description: 'Preferência criada com sucesso',
  })
  @HttpCode(HttpStatus.CREATED)
  @Post('create-preferences')
  createPreferences(@Body() body: { productId: string }) {
    return this.paymentService.createPreferences(body.productId);
  }

  @ApiOperation({
    summary: 'Cria pagamento via PIX',
    description:
      'Gera QR code e código copia-e-cola para pagamento via PIX',
  })
  @ApiResponse({
    status: 201,
    description: 'Pagamento PIX criado com QR code',
  })
  @ApiResponse({
    status: 400,
    description: 'Dados inválidos',
  })
  @HttpCode(HttpStatus.CREATED)
  @Post('pix')
  createPixPayment(@Body() dto: CreatePixPaymentDto) {
    return this.paymentService.createPixPayment(dto);
  }

  @ApiOperation({
    summary: 'Webhook do Mercado Pago',
    description: 'Webhook do Mercado Pago para processar pagamentos',
  })
  @ApiResponse({
    status: 200,
    description: 'Webhook processado com sucesso',
  })
  @HttpCode(HttpStatus.OK)
  @Post('webhook')
  webhook(@Body() body: any) {
    return this.paymentService.handleWebhook(body);
  }
}
