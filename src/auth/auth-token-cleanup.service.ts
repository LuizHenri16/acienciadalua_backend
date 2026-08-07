import { Injectable, Logger } from '@nestjs/common';
import { Cron, CronExpression } from '@nestjs/schedule';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class AuthTokenCleanupService {
  private readonly logger = new Logger(AuthTokenCleanupService.name);

  constructor(private readonly prismaService: PrismaService) {}

  // Executa a cada hora para limpar tokens expirados ou já utilizados
  @Cron(CronExpression.EVERY_DAY_AT_MIDNIGHT)
  async handleCron() {
    this.logger.log(
      'Iniciando a limpeza de tokens (Magic Links) expirados ou usados...',
    );

    try {
      // Remove tokens que expiraram ou foram usados
      const result = await this.prismaService.authToken.deleteMany({
        where: {
          OR: [
            { expiresAt: { lt: new Date() } }, // Verifica se o token expirou
            { usedAt: { not: null as any } }, // Verifica se o token foi usado
          ],
        },
      });

      this.logger.log(
        `Limpeza concluída. ${result.count} tokens removidos do banco de dados.`,
      );
    } catch (error) {
      this.logger.error('Falha ao limpar tokens do banco de dados', error);
    }
  }
}
