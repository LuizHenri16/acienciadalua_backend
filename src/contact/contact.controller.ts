import { Body, Controller, HttpCode, Post } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { EmailService } from '../email/email.service';

class ContactDTO {
  name: string;
  email: string;
  subject?: string;
  message: string;
}

@ApiTags('Contact')
@Controller('contact')
export class ContactController {
  constructor(private readonly emailService: EmailService) {}

  @Post()
  @HttpCode(200)
  @ApiOperation({ summary: 'Send a contact message from the website form' })
  @ApiBody({
    schema: {
      type: 'object',
      required: ['name', 'email', 'message'],
      properties: {
        name: { type: 'string' },
        email: { type: 'string' },
        subject: { type: 'string' },
        message: { type: 'string' },
      },
    },
  })
  @ApiResponse({ status: 200, description: 'Message sent successfully.' })
  @ApiResponse({ status: 400, description: 'Validation error.' })
  @ApiResponse({ status: 500, description: 'Failed to send message.' })
  async sendContact(@Body() body: ContactDTO) {
    await this.emailService.sendContactEmail(
      body.name,
      body.email,
      body.subject ?? '',
      body.message,
    );

    return { message: 'Mensagem enviada com sucesso!' };
  }
}
