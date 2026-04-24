import { Injectable, UnauthorizedException } from '@nestjs/common';
import { SignInDTO, SignUpDTO } from './dtos/auth';
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

        if (userAlreadyExists) {
            throw new UnauthorizedException("User already exists")
        }

        const user = await this.prismaSevice.user.create({data})

        return {
            id: user.id,
            email: user.email,
            name: user.name
        }
    }

    async signin(data: SignInDTO) {
        const userExists = await this.prismaSevice.user.findUnique({
            where: {
                email: data.email,
                password: data.password
            }
        })

        if (!userExists) {
            throw new UnauthorizedException("User email or password wrong.");
        } 

        return {
            id: userExists.id,
            name: userExists.name,
            email: userExists.email
        }

    }
}
