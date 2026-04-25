import { Body, Controller, HttpCode, Post } from '@nestjs/common';
import { AuthService } from './auth.service';
import { RefreshDTO, SignInDTO, SignUpDTO } from './dtos/auth';

@Controller('auth')
export class AuthController {

    constructor(private readonly authService: AuthService) {}

    @Post('/signup')
    async signUp(@Body() signUpDTO: SignUpDTO) {
        const tokens = await this.authService.signup(signUpDTO);
        return {
            message: "User created successfully",
            ...tokens
        };
    }

    @Post('/signin')
    @HttpCode(200)
    async signIn(@Body() signInDTO: SignInDTO) {
        const tokens = await this.authService.signin(signInDTO);
        return {
            message: "User signed in successfully",
            ...tokens
        };
    }

    @Post('/refresh')
    @HttpCode(200)
    async refresh(@Body() refreshDTO: RefreshDTO) {
        const tokens = await this.authService.refresh(refreshDTO.refresh_token);
        return {
            message: "Token refreshed successfully",
            ...tokens
        };
    }
}
