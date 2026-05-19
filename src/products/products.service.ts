import { Injectable, NotFoundException } from '@nestjs/common';
import { PrismaService } from '../prisma/prisma.service';
import { CreateProductDTO } from './dtos/create-product.dto';
import { UpdateProductDTO } from './dtos/update-product.dto';
import { Category } from '@prisma/client';

@Injectable()
export class ProductsService {
  constructor(private readonly prisma: PrismaService) {}

  async findAll(category?: Category, isActive?: boolean) {
    return this.prisma.product.findMany({
      where: {
        ...(category && { category }),
        ...(isActive !== undefined && { isActive }),
      },
      orderBy: { createdAt: 'desc' },
    });
  }

  async findOne(id: string) {
    const product = await this.prisma.product.findUnique({ where: { id } });
    if (!product) throw new NotFoundException('Product not found.');
    return product;
  }

  async create(dto: CreateProductDTO, coverUrl: string, fileUrl: string) {
    return this.prisma.product.create({
      data: {
        ...dto,
        coverUrl,
        fileUrl,
      },
    });
  }

  async update(id: string, dto: UpdateProductDTO, coverUrl?: string, fileUrl?: string) {
    await this.findOne(id);
    return this.prisma.product.update({
      where: { id },
      data: {
        ...dto,
        ...(coverUrl && { coverUrl }),
        ...(fileUrl && { fileUrl }),
      },
    });
  }

  async toggle(id: string) {
    const product = await this.findOne(id);
    return this.prisma.product.update({
      where: { id },
      data: { isActive: !product.isActive },
    });
  }

  async remove(id: string) {
    await this.findOne(id);
    return this.prisma.product.delete({ where: { id } });
  }
}
