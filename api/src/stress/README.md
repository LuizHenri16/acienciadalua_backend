# Teste de estresse da API

Programa Go (somente biblioteca padrão) que envia requisições HTTP contra uma
instância da API e reporta latência, throughput, códigos de resposta e erros.
Rode-o a partir de `api/src` com a API já em execução.

```bash
cd acienciadalua_backend/api/src

# Leitura (padrão, sem escritas): 1000 requisições, 10 workers
go run ./stress

# 20 conexões, 30 segundos, contra a API Go na 3001
go run ./stress -base-url=http://localhost:3001 -concurrency=20 -duration=30s

# Webhooks inofensivos
go run ./stress -scenario=webhook -requests=2000

# Ciclo completo de produto (requer admin; veja os avisos abaixo)
go run ./stress -scenario=crud -admin-token="$TOKEN" -requests=100

# Falha o processo se mais de 1% das respostas forem erro de transporte/5xx
go run ./stress -scenario=read -requests=5000 -max-error-rate=0.01
```

O comando encerra com `0` em sucesso, `2` em opções inválidas e `1` em falha de
setup/execução ou quando `-max-error-rate` é excedido.

## Flags

| Flag | Padrão | Descrição |
|---|---|---|
| `-base-url` | `http://localhost:3000` | URL base da API |
| `-scenario` | `read` | `read`, `webhook`, `auth`, `crud` ou `mixed` |
| `-concurrency` | `10` | Workers simultâneos (cada um mantém conexões reutilizadas) |
| `-requests` | `1000` | Total de requisições (`0` = usar só `-duration`) |
| `-duration` | `0` | Duração máxima, ex.: `30s`, `2m` |
| `-rate` | `0` | Limite global de requisições/segundo (`0` = sem limite) |
| `-timeout` | `10s` | Timeout por requisição |
| `-product-id` | — | Produto incluído no cenário `read` |
| `-admin-email` / `-admin-password` | — | Credenciais do admin (`auth` e `crud`) |
| `-admin-token` | — | Access token do admin (evita o signin no setup do `crud`) |
| `-customer-email` / `-customer-password` | — | Credenciais do cliente (`auth`) |
| `-distribute-ip` | `false` | Envia um `X-Forwarded-For` único por requisição |
| `-json` | `false` | Relatório em JSON (útil para CI) |
| `-max-error-rate` | `0` | Falha se a taxa de erro (transporte + 5xx) exceder, ex.: `0.01` |
| `-allow-remote-writes` | `false` | Confirma escritas destrutivas em host remoto |

Quando `-duration` é informado sem `-requests`, o limite de requisições deixa de
ser aplicado. Se ambos forem informados, o teste para no primeiro limite
atingido. `-rate` funciona como um teto global compartilhado pelos workers.

## Cenários

| Cenário | O que faz | Escreve? |
|---|---|---|
| `read` | GETs em `/health`, `/products`, filtros de categoria/ativo e, se `-product-id` for passado, `/products/:id` | Não |
| `webhook` | POSTs em `/payment/webhook` com notificações que o handler ignora (sem consultar o Mercado Pago) | Não |
| `auth` | POSTs em `/auth/admin/signin` e/ou `/auth/user/signin` | Não |
| `crud` | Ciclo `POST → PUT → PATCH toggle → GET → DELETE` em `/products` | Sim |
| `mixed` | 90% `read` + 10% `webhook` | Não |

### `auth` e rate limit

As rotas de autenticação têm rate limit por IP (10 req/min no admin e no
`/auth/user/signin`; 3 req/15min no forgot-password). Com `auth`, as primeiras
requisições passam e o restante tende a receber **429** — o relatório mostra um
aviso nesse caso.

`-distribute-ip` envia um `X-Forwarded-For` diferente por requisição. O
middleware `RealIP` do chi confia nesse cabeçalho, então o limite é aplicado por
"IP" e o caminho de signin é medido de verdade. Use **apenas** em ambiente
local/staging controlado: em produção isso contorna a proteção de rate limit.

### `crud`: escritas e limpeza

O cenário `crud` cria produtos reais. Para evitar acidentes:

- exige `-admin-token` ou `-admin-email`/`-admin-password`;
- recusa hosts que não sejam loopback (`localhost`, `127.0.0.1`, `::1`,
  `0.0.0.0`) a menos que `-allow-remote-writes` seja passado;
- usa títulos únicos (`stress-<timestamp>-<worker>-<seq>`);
- rastreia todos os IDs criados e remove os que sobrarem no `Cleanup`, inclusive
  quando a execução é interrompida com Ctrl+C/SIGTERM.

Ainda assim, rode contra um banco descartável: falhas de rede podem impedir a
remoção de algum produto. O `Cleanup` tenta novamente no encerramento.

## Relatório

O resumo humano traz requisições, duração, throughput, bytes recebidos,
distribuição de status, erros de transporte, latências (mín/média/p50/p90/p95/
p99/máx), taxa de sucesso (2xx/3xx) e taxa de erro (transporte + 5xx). Respostas
4xx não contam como erro: em `auth`, por exemplo, 429 é um resultado esperado.

Com `-json`:

```json
{
  "base_url": "http://localhost:3000",
  "scenario": "read",
  "concurrency": 20,
  "requests": 5000,
  "duration_seconds": 12.34,
  "throughput_rps": 405.1,
  "bytes_received": 1234567,
  "status_codes": { "200": 5000 },
  "transport_errors": 0,
  "build_errors": 0,
  "latency_ms": { "min": 0.4, "avg": 2.1, "p50": 1.8, "p90": 3.5, "p95": 4.2, "p99": 8.0, "max": 25.0 },
  "success_rate": 1,
  "error_rate": 0
}
```

Para usar em CI, combine `-json -max-error-rate=<limite>` e valide o código de
saída:

```bash
go run ./stress -scenario=read -requests=3000 -json -max-error-rate=0.01 > report.json
```

## Limitações

- A carga é gerada por um único processo (modelo closed-loop por worker); para
  saturar várias máquinas, execute o programa em paralelo com `-rate` dividido.
- O relatório agrega apenas latência e status; não valida o corpo das respostas
  (exceto o `id` criado no `crud`).
- `-distribute-ip` pressupõe que a API confie em `X-Forwarded-For` (é o caso do
  router atual). Atrás de um proxy que sobrescreve o cabeçalho, o efeito pode
  ser diferente.

## Testes do próprio programa

Os testes usam `httptest`, não precisam de banco nem de API no ar:

```bash
go test ./stress/...
```
