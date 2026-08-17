import { Injectable, NotFoundException } from '@nestjs/common';
import { existsSync } from 'fs';
import { join } from 'path';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class CustomersService {
  constructor(private readonly prisma: PrismaService) {}

  async getMe(customerId: string) {
    const customer = await this.prisma.customer.findUnique({
      where: { id: customerId },
    });

    if (!customer) throw new NotFoundException('Customer not found.');

    return customer;
  }

  async getMyPurchases(customerId: string) {
    const purchases = await this.prisma.purchase.findMany({
      where: { customerId },
      include: {
        product: {
          select: {
            id: true,
            title: true,
            description: true,
            coverUrl: true,
            fileUrl: true,
            category: true,
          },
        },
      },
      orderBy: { purchasedAt: 'desc' },
    });

    const mapPurchase = (p: (typeof purchases)[0]) => ({
      ...p.product,
      purchasedAt: p.purchasedAt,
    });

    return {
      student: purchases
        .filter((p) => p.product.category === 'STUDENT')
        .map(mapPurchase),
      teacher: purchases
        .filter((p) => p.product.category === 'TEACHER')
        .map(mapPurchase),
    };
  }

  async getDownload(customerId: string, productId: string) {
    const purchase = await this.prisma.purchase.findFirst({
      where: { customerId, productId },
      include: { product: true },
    });

    if (!purchase) {
      throw new NotFoundException('Compra não encontrada.');
    }

    const fileUrl = purchase.product.fileUrl;
    if (!fileUrl) {
      throw new NotFoundException('Produto sem arquivo.');
    }

    const filePath = join(process.cwd(), 'files', fileUrl);
    const legacyPath = join(process.cwd(), 'uploads', fileUrl);
    const path = existsSync(filePath)
      ? filePath
      : existsSync(legacyPath)
        ? legacyPath
        : null;

    if (!path) {
      throw new NotFoundException('Arquivo não encontrado no servidor.');
    }

    return { product: purchase.product, path, fileUrl };
  }
}
