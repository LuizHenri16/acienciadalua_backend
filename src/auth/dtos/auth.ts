import { IsEmail, IsNotEmpty, IsOptional, IsString, MinLength } from 'class-validator';

export class SignUpDTO {
    @IsString({ message: "Name must be a text" })
    @IsNotEmpty({ message: "Name is required" })
    @MinLength(3, { message: "Name must be at least 3 characters" })
    name: string;

    @IsEmail({}, { message: "Invalid email format" })
    @IsNotEmpty({ message: "Email is required" })
    email: string;

    @IsString({ message: "Role must be a text" })
    @IsOptional()
    role?: string;

    @IsString({ message: "Password must be a text" })
    @IsNotEmpty({ message: "Password is required" })
    @MinLength(8, { message: "Password must be at least 8 characters" })
    password: string;
}

export class SignInDTO {
    @IsEmail({}, { message: "Invalid email format" })
    @IsNotEmpty({ message: "Email is required" })
    email: string;

    @IsString({ message: "Password must be a text" })
    @IsNotEmpty({ message: "Password is required" })
    @MinLength(8, { message: "Password must be at least 8 characters" })
    password: string;
}

export class RefreshDTO {
    @IsString({ message: "Refresh token must be a text" })
    @IsNotEmpty({ message: "Refresh token is required" })
    refresh_token: string;
}
