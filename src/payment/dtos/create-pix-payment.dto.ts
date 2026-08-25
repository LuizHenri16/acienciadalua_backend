import { ApiProperty } from '@nestjs/swagger';
import { Type } from 'class-transformer';
import {
  IsEmail,
  IsNotEmpty,
  IsOptional,
  IsString,
  ValidateNested,
} from 'class-validator';

class PixPayerDto {
  @ApiProperty({ example: 'comprador@email.com' })
  @IsEmail()
  @IsNotEmpty()
  email: string;

  @ApiProperty({ example: 'João Silva', required: false })
  @IsString()
  @IsOptional()
  name?: string;

  @ApiProperty({ example: '12345678909', required: false })
  @IsString()
  @IsOptional()
  cpf?: string;
}

export class CreatePixPaymentDto {
  @ApiProperty({ example: 'uuid-do-produto' })
  @IsString()
  @IsNotEmpty()
  productId: string;

  @ApiProperty({ type: PixPayerDto })
  @ValidateNested()
  @Type(() => PixPayerDto)
  @IsNotEmpty()
  payer: PixPayerDto;
}
