import { Body, Controller, HttpCode, Post } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { AuthService } from './auth.service';
import { RefreshDTO, SignInDTO, SignUpDTO } from './dtos/auth';

@ApiTags('Auth')
@Controller('auth')
export class AuthController {

    constructor(private readonly authService: AuthService) { }

    @Post('/signup')
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

    @Post('/signin')
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

    @Post('/refresh')
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
}
