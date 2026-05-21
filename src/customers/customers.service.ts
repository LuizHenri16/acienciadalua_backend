import { Injectable, NotFoundException } from '@nestjs/common';
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

    const mapPurchase = (p: typeof purchases[0]) => ({
      ...p.product,
      purchasedAt: p.purchasedAt,
    });

    return {
      student: purchases.filter(p => p.product.category === 'STUDENT').map(mapPurchase),
      teacher: purchases.filter(p => p.product.category === 'TEACHER').map(mapPurchase),
    };
  }
}
