export interface SignUpDTO {
    name: string;
    email: string;
    role: string;
    password: string;
}

export interface SignInDTO {
    email: string;
    password: string;
}
