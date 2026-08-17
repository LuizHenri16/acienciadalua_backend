import {
  Controller,
  Get,
  Param,
  Request,
  Res,
  StreamableFile,
  UseGuards,
} from '@nestjs/common';
import { createReadStream } from 'fs';
import { extname } from 'path';
import type { Response } from 'express';
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

  @Get('me/downloads/:productId')
  @ApiOperation({
    summary: 'Baixa o arquivo de um produto comprado',
  })
  @ApiResponse({
    status: 200,
    description: 'Arquivo enviado para download',
  })
  @ApiResponse({
    status: 404,
    description: 'Compra ou arquivo não encontrado',
  })
  async downloadProductFile(
    @Request() req: any,
    @Param('productId') productId: string,
    @Res({ passthrough: true }) res: Response,
  ) {
    const { path, fileUrl } =
      await this.customersService.getDownload(req.customer.sub, productId);

    const ext = extname(fileUrl).toLowerCase();
    const mime =
      ext === '.pdf'
        ? 'application/pdf'
        : ext === '.zip'
          ? 'application/zip'
          : 'application/octet-stream';

    res.set({
      'Content-Type': mime,
      'Content-Disposition': `attachment; filename="${fileUrl.replace(/[^\w.\-]/g, '_')}"`,
      'Cache-Control': 'no-store',
    });

    return new StreamableFile(createReadStream(path));
  }
}
