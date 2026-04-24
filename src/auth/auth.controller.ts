import { Body, Controller, Post } from '@nestjs/common';
import { AuthService } from './auth.service';
import type { SignInDTO, SignUpDTO } from './dtos/auth';

@Controller('auth')
export class AuthController {

        constructor(private readonly authService: AuthService) {}

        @Post('/signup')
        async signUp(@Body() signUpDTO: SignUpDTO) {
            return await this.authService.signup(signUpDTO);
        }

        @Post('/signin')
        async signIn(@Body() signInDTO: SignInDTO) {
            return await this.authService.signin(signInDTO)
        }
}
