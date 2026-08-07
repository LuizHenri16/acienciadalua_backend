import {
  IsEmail,
  IsNotEmpty,
  IsOptional,
  IsString,
  MinLength,
} from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

export class SignUpDTO {
  @ApiProperty({
    example: 'Luana Araujo',
    description: 'Full name of the user',
  })
  @IsString({ message: 'Name must be a text' })
  @IsNotEmpty({ message: 'Name is required' })
  @MinLength(3, { message: 'Name must be at least 3 characters' })
  name: string;

  @ApiProperty({
    example: 'luana@gmail.com',
    description: 'User email address',
  })
  @IsEmail({}, { message: 'Invalid email format' })
  @IsNotEmpty({ message: 'Email is required' })
  email: string;

  @ApiPropertyOptional({
    example: 'admin',
    description: 'User role (default: colaborator)',
  })
  @IsString({ message: 'Role must be a text' })
  @IsOptional()
  role?: string;

  @ApiProperty({
    example: 'password123',
    description: 'Password with at least 8 characters',
  })
  @IsString({ message: 'Password must be a text' })
  @IsNotEmpty({ message: 'Password is required' })
  @MinLength(8, { message: 'Password must be at least 8 characters' })
  password: string;
}

export class SignInDTO {
  @ApiProperty({
    example: 'luana@gmail.com',
    description: 'User email address',
  })
  @IsEmail({}, { message: 'Invalid email format' })
  @IsNotEmpty({ message: 'Email is required' })
  email: string;

  @ApiProperty({ example: 'password123', description: 'User password' })
  @IsString({ message: 'Password must be a text' })
  @IsNotEmpty({ message: 'Password is required' })
  @MinLength(8, { message: 'Password must be at least 8 characters' })
  password: string;
}

export class RefreshDTO {
  @ApiProperty({
    example: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...',
    description: 'Valid refresh token',
  })
  @IsString({ message: 'Refresh token must be a text' })
  @IsNotEmpty({ message: 'Refresh token is required' })
  refresh_token: string;
}

export class CustomerSigninDTO {
  @ApiProperty({
    example: 'luana@gmail.com',
    description: 'User email address',
  })
  @IsEmail({}, { message: 'Invalid email format' })
  @IsNotEmpty({ message: 'Email is required' })
  email: string;

  @ApiProperty({ example: 'password123', description: 'User password' })
  @IsString({ message: 'Password must be a text' })
  @IsNotEmpty({ message: 'Password is required' })
  password: string;
}

export class CustomerSetPasswordDTO {
  @ApiProperty({ example: 'luana@gmail.com', description: 'Customer email' })
  @IsEmail({}, { message: 'Invalid email format' })
  @IsNotEmpty({ message: 'Email is required' })
  email: string;

  @ApiProperty({
    example: 'password123',
    description: 'Password with at least 8 characters',
  })
  @IsString({ message: 'Password must be a text' })
  @IsNotEmpty({ message: 'Password is required' })
  @MinLength(8, { message: 'Password must be at least 8 characters' })
  password: string;
}

export class ForgotPasswordDTO {
  @ApiProperty({
    example: 'luana@gmail.com',
    description: 'User email address',
  })
  @IsEmail({}, { message: 'Invalid email format' })
  @IsNotEmpty({ message: 'Email is required' })
  email: string;
}

export class ResetPasswordDTO {
  @ApiProperty({
    example: 'uuid-token',
    description: 'Reset token received by email',
  })
  @IsString({ message: 'Token must be a text' })
  @IsNotEmpty({ message: 'Token is required' })
  token: string;

  @ApiProperty({
    example: 'newpassword123',
    description: 'New password with at least 8 characters',
  })
  @IsString({ message: 'Password must be a text' })
  @IsNotEmpty({ message: 'Password is required' })
  @MinLength(8, { message: 'Password must be at least 8 characters' })
  password: string;
}
