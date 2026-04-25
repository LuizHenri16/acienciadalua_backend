import { Body, Controller, HttpCode, Post } from '@nestjs/common';
import { AuthService } from './auth.service';
import type { SignInDTO, SignUpDTO } from './dtos/auth';

@Controller('auth')
export class AuthController {

        constructor(private readonly authService: AuthService) {}

        @Post('/signup')
        async signUp(@Body() signUpDTO: SignUpDTO) {
            const token = await this.authService.signup(signUpDTO)
            return {
                message: "User created successfully",
                token
            };
        }

        @Post('/signin')
        @HttpCode(200)
        async signIn(@Body() signInDTO: SignInDTO) {
            const token = await this.authService.signin(signInDTO)
            return {
                message: "User signed in successfully",
                token
            };
        }
}
