import { ApiProperty } from '@nestjs/swagger';
import { Category } from '@prisma/client';
import { Transform, Type } from 'class-transformer';
import { IsBoolean, IsEnum, IsNotEmpty, IsNumber, IsString, Min } from 'class-validator';

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

    @ApiProperty({ example: true })
    @IsBoolean()
    @Transform(({ value }) => value === 'true' || value === true)
    isActive: boolean;

    @ApiProperty({ enum: Category, example: Category.STUDENT })
    @IsEnum(Category)
    category: Category;
}
