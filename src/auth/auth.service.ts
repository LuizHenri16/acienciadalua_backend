import { Injectable, UnauthorizedException } from '@nestjs/common';
import { SignInDTO, SignUpDTO, SignUpResponseDTO } from './dtos/auth';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class AuthService {
    constructor (private prismaSevice: PrismaService) {}
    
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

        // else, create the user and return the user data without the password for security reasons
        const user = await this.prismaSevice.user.create({data})
        return {
            id: user.id,
            name: user.name,
            email: user.email,
            role: user.role
        } as SignUpResponseDTO
    }

    async signin(data: SignInDTO) {
        const userExists = await this.prismaSevice.user.findUnique({
            where: {
                email: data.email,
                password: data.password
            }
        })

        // If user does not exists, throw an unauthorized exception with a message
        if (!userExists) {
            throw new UnauthorizedException("User email or password wrong.");
        } 

        // else, return the user data without the password for security reasons
        return {
            id: userExists.id,
            name: userExists.name,
            email: userExists.email
        }

    }
}
