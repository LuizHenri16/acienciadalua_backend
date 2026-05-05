import { Body, Controller, Get, HttpCode, Post, Query, Res } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { AuthService } from './auth.service';
import { CustomerSigninDTO, RefreshDTO, SignInDTO, SignUpDTO } from './dtos/auth';
import type { Response } from 'express';

@ApiTags('Auth')
@Controller('auth')
export class AuthController {

    constructor(private readonly authService: AuthService) { }

    @Post('admin/signup')
    @ApiOperation({ summary: 'Register a new user' })
    @ApiBody({ type: SignUpDTO })
    @ApiResponse({ status: 201, description: 'User created successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'User already exists.' })
    async signUp(@Body() signUpDTO: SignUpDTO) {
        const tokens = await this.authService.signup(signUpDTO);
        return {
            message: "User created successfully",
            ...tokens
        };
    }

    @Post('admin/signin')
    @HttpCode(200)
    @ApiOperation({ summary: 'Authenticate user and return tokens' }) // description of the route
    @ApiBody({ type: SignInDTO })
    @ApiResponse({ status: 200, description: 'User signed in successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'Invalid email or password.' })
    async signIn(@Body() signInDTO: SignInDTO) {
        const tokens = await this.authService.signin(signInDTO);
        return {
            message: "User signed in successfully",
            ...tokens
        };
    }

    @Post('admin/refresh')
    @HttpCode(200)
    @ApiOperation({ summary: 'Generate a new access_token using a refresh_token' }) // description of the route
    @ApiBody({ type: RefreshDTO })
    @ApiResponse({ status: 200, description: 'New access_token generated successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'Invalid or expired refresh token.' })
    async refresh(@Body() refreshDTO: RefreshDTO) {
        const tokens = await this.authService.refresh(refreshDTO.refresh_token);
        return {
            message: "Token refreshed successfully",
            ...tokens
        };
    }

    @Post('user/signin')
    @HttpCode(200)
    @ApiOperation({ summary: 'Authenticate user and return tokens' }) // description of the route
    @ApiBody({ type: CustomerSigninDTO })
    @ApiResponse({ status: 200, description: 'User signed in successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'Invalid email or password.' })
    async customerGenerateMagicLink(@Body() customerSigninDTO: CustomerSigninDTO) {
        await this.authService.customerGenerateMagicLink(customerSigninDTO);
        return {
            message: "Magic link sent successfully",
        };
    }

    @Get('verify-link')
    @HttpCode(200)
    @ApiOperation({ summary: 'Verify magic link and return tokens' }) // description of the route
    @ApiResponse({ status: 200, description: 'Magic link verified successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'Invalid or expired magic link.' })
    async verifyMagicLink(@Query('token') token: string, @Res() res: Response) {
        const jwt = await this.authService.customerVerifyMagicLink(token);

        return res.redirect(`${process.env.APP_URL}/login?access_token=${jwt}`);
    }
}
