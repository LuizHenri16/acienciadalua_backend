import { JwtService } from '@nestjs/jwt';
import { Injectable, UnauthorizedException } from '@nestjs/common';
import { SignInDTO, SignUpDTO, SignUpResponseDTO } from './dtos/auth';
import { PrismaService } from '../prisma/prisma.service';
import { PasswordService } from './password.service';

@Injectable()
export class AuthService {
    constructor (
        private prismaSevice: PrismaService,
        private jwtService: JwtService,
        private passwordService: PasswordService
    ) {}
    
    async signup(data: SignUpDTO) {

        const userAlreadyExists = await this.prismaSevice.user.findUnique({
            where: {
                email: data.email
            }
        });

        // If user already exists, throw an unauthorized exception because the exception returns a status code 401 and a message
        if (userAlreadyExists) {
            throw new UnauthorizedException("User already exists")
        }

        // If user does not exists, hash the password and create the user in the database with the hashed password, then return the user data without the password for security reasons
        const hashedPassword = await this.passwordService.hashPassword(data.password);
        data.password = hashedPassword;

        // else, create the user and return the user data without the password for security reasons
        const user = await this.prismaSevice.user.create({data});

        // Generate a JWT token for the user with the user id, email and role as payload and return it in the response
        const token = this.jwtService.sign({ id: user.id, email: user.email, role: user.role });

        return {
            token
        }
    }

    async signin(data: SignInDTO) {
        const userExists = await this.prismaSevice.user.findUnique({
            where: {
                email: data.email  
            }
        })

        // If user does not exists, throw an unauthorized exception with a message
        if (!userExists || !(await this.passwordService.comparePassword(data.password, userExists.password))) {
            throw new UnauthorizedException("User email or password wrong.");
        } 

        const token = this.jwtService.sign({ id: userExists.id, email: userExists.email, role: userExists.role });
        // else, return the user data without the password for security reasons
        return {
            token
        }

    }
}
