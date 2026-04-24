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

export interface SignUpResponseDTO {
    id: string;
    name: string;
    email: string;
    role: string;
}

export interface SignInResponseDTO {
    id: string;
    name: string;
    email: string;
    role: string;
}