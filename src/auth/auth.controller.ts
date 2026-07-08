import { Body, Controller, HttpCode, Post, UseGuards } from '@nestjs/common';
import { ThrottlerGuard, Throttle } from '@nestjs/throttler';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { AuthService } from './auth.service';
import { CustomerSigninDTO, CustomerSetPasswordDTO, ForgotPasswordDTO, ResetPasswordDTO, RefreshDTO, SignInDTO, SignUpDTO } from './dtos/auth';

@ApiTags('Auth')
@Controller('auth')
@UseGuards(ThrottlerGuard)
export class AuthController {

    constructor(private readonly authService: AuthService) { }

    // ─── Admin ──────────────────────────────────────────────────────────────────

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
    @ApiOperation({ summary: 'Authenticate user and return tokens' })
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
    @ApiOperation({ summary: 'Generate a new access_token using a refresh_token' })
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

    // ─── Customer (password-based) ──────────────────────────────────────────────

    @Post('user/signin')
    @HttpCode(200)
    @Throttle({ default: { limit: 10, ttl: 900000 } })
    @ApiOperation({ summary: 'Sign in with email and password' })
    @ApiBody({ type: CustomerSigninDTO })
    @ApiResponse({ status: 200, description: 'User signed in successfully.' })
    @ApiResponse({ status: 400, description: 'Validation error.' })
    @ApiResponse({ status: 401, description: 'Invalid email/password or no password set.' })
    async customerSignIn(@Body() dto: CustomerSigninDTO) {
        const result = await this.authService.customerSignIn(dto);
        return {
            message: "Signed in successfully",
            ...result
        };
    }

    @Post('user/password')
    @HttpCode(200)
    @ApiOperation({ summary: 'Set initial password (first access)' })
    @ApiBody({ type: CustomerSetPasswordDTO })
    @ApiResponse({ status: 200, description: 'Password set successfully.' })
    @ApiResponse({ status: 404, description: 'Customer not found.' })
    @ApiResponse({ status: 400, description: 'Password already set or validation error.' })
    async customerSetPassword(@Body() dto: CustomerSetPasswordDTO) {
        const result = await this.authService.customerSetPassword(dto);
        return {
            message: "Password set successfully",
            ...result
        };
    }

    @Post('user/forgot-password')
    @HttpCode(200)
    @Throttle({ default: { limit: 3, ttl: 900000 } })
    @ApiOperation({ summary: 'Send reset password email' })
    @ApiBody({ type: ForgotPasswordDTO })
    @ApiResponse({ status: 200, description: 'Reset email sent if email exists.' })
    async customerForgotPassword(@Body() dto: ForgotPasswordDTO) {
        await this.authService.customerForgotPassword(dto);
        return {
            message: "If the email exists, a reset link has been sent.",
        };
    }

    @Post('user/reset-password')
    @HttpCode(200)
    @ApiOperation({ summary: 'Reset password using token from email' })
    @ApiBody({ type: ResetPasswordDTO })
    @ApiResponse({ status: 200, description: 'Password reset successfully.' })
    @ApiResponse({ status: 400, description: 'Invalid or expired token.' })
    async customerResetPassword(@Body() dto: ResetPasswordDTO) {
        const result = await this.authService.customerResetPassword(dto);
        return {
            message: "Password reset successfully",
            ...result
        };
    }

}
