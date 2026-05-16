import { ApiProperty } from '@nestjs/swagger';
import { Category } from '@prisma/client';
import { Type } from 'class-transformer';
import { IsEnum, IsNotEmpty, IsNumber, IsString, Min } from 'class-validator';

export class CreateProductDTO {
    @ApiProperty({ example: 'Apostila de Química' })
    @IsString()
    @IsNotEmpty()
    title: string;

    @ApiProperty({ example: 'Material completo para o ENEM.' })
    @IsString()
    @IsNotEmpty()
    description: string;

    @ApiProperty({ example: 49.90 })
    @Type(() => Number)
    @IsNumber()
    @Min(0)
    price: number;

    @ApiProperty({ enum: Category, example: Category.STUDENT })
    @IsEnum(Category)
    category: Category;
}
