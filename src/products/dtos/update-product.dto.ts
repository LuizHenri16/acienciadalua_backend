import { ApiPropertyOptional } from '@nestjs/swagger';
import { Category } from '@prisma/client';
import { Type } from 'class-transformer';
import { IsEnum, IsNumber, IsOptional, IsString, Min } from 'class-validator';

export class UpdateProductDTO {
    @ApiPropertyOptional({ example: 'Apostila de Química' })
    @IsString()
    @IsOptional()
    title?: string;

    @ApiPropertyOptional({ example: 'Material completo para o ENEM.' })
    @IsString()
    @IsOptional()
    description?: string;

    @ApiPropertyOptional({ example: 49.90 })
    @Type(() => Number)
    @IsNumber()
    @Min(0)
    @IsOptional()
    price?: number;

    @ApiPropertyOptional({ enum: Category, example: Category.STUDENT })
    @IsEnum(Category)
    @IsOptional()
    category?: Category;
}
