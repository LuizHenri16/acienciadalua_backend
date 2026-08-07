import { Controller, Get, Request, UseGuards } from '@nestjs/common';
import {
  ApiBearerAuth,
  ApiOperation,
  ApiResponse,
  ApiTags,
} from '@nestjs/swagger';
import { CustomersService } from './customers.service';
import { JwtCustomerGuard } from '../jwt/jwt-customer.guard';

@ApiTags('Customers')
@ApiBearerAuth()
@UseGuards(JwtCustomerGuard)
@Controller('customers')
export class CustomersController {
  constructor(private readonly customersService: CustomersService) {}

  @Get('me')
  @ApiOperation({ summary: 'Retorna os dados do customer autenticado' })
  @ApiResponse({ status: 200, description: 'Dados do customer' })
  getMe(@Request() req: any) {
    return this.customersService.getMe(req.customer.sub);
  }

  @Get('me/purchases')
  @ApiOperation({
    summary: 'Retorna as compras do customer separadas por categoria',
  })
  @ApiResponse({
    status: 200,
    description: 'Materiais separados em student e teacher',
  })
  getMyPurchases(@Request() req: any) {
    return this.customersService.getMyPurchases(req.customer.sub);
  }
}
