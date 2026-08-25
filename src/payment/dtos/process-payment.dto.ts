import { ApiProperty } from '@nestjs/swagger';
import { Type } from 'class-transformer';
import {
  IsEmail,
  IsNotEmpty,
  IsNumber,
  IsOptional,
  IsString,
  Min,
  ValidateNested,
} from 'class-validator';

class IdentificationDto {
  @ApiProperty({ example: 'CPF' })
  @IsString()
  @IsNotEmpty()
  type: string;

  @ApiProperty({ example: '12345678909' })
  @IsString()
  @IsNotEmpty()
  number: string;
}

class PayerDto {
  @ApiProperty({ example: 'comprador@email.com' })
  @IsEmail()
  @IsNotEmpty()
  email: string;

  @ApiProperty({ example: 'João Silva', required: false })
  @IsString()
  @IsOptional()
  name?: string;

  @ApiProperty({ type: IdentificationDto, required: false })
  @ValidateNested()
  @Type(() => IdentificationDto)
  @IsOptional()
  identification?: IdentificationDto;
}

export class ProcessPaymentDto {
  @ApiProperty({ example: 'card_token_gerado_pelo_mp' })
  @IsString()
  @IsNotEmpty()
  token: string;

  @ApiProperty({ example: 'visa' })
  @IsString()
  @IsNotEmpty()
  payment_method_id: string;

  @ApiProperty({ example: 1 })
  @Type(() => Number)
  @IsNumber()
  @Min(1)
  installments: number;

  @ApiProperty({ example: '310' })
  @IsString()
  @IsNotEmpty()
  issuer_id: string;

  @ApiProperty({ example: 'uuid-do-produto' })
  @IsString()
  @IsNotEmpty()
  productId: string;

  @ApiProperty({ type: PayerDto })
  @ValidateNested()
  @Type(() => PayerDto)
  @IsNotEmpty()
  payer: PayerDto;
}
