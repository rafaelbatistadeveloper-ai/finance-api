# Design: MVP de Controle Financeiro (Custo de Vida)

Este documento descreve como os requisitos de `requirements.md` serão implementados. Cada decisão cita o requisito que a motivou.

## 1. Visão geral

API REST em Go, com PostgreSQL. Sem frontend no MVP: a API é exercitada via OpenAPI/Swagger e `curl`. Uma interface fica para depois.

### Stack
| Item | Escolha | Motivo |
|---|---|---|
| Linguagem | Go 1.22+ | stack do autor; `net/http` já tem roteamento por método e padrão |
| Framework HTTP | Gin (`gin-gonic/gin`) | binding e validação de JSON, grupos de rotas, middlewares, grande adoção e material de estudo |
| Banco | PostgreSQL 16 | consultas de agregação e integridade relacional |
| Acesso a dados | `pgx` + `sqlc` | SQL explícito e tipado, sem ORM |
| Migrations | `goose` | simples e versionadas |
| Auth | JWT (HS256) + bcrypt | R1 |
| Config | variáveis de ambiente | segredos fora do código |
| Testes | `testing` + `testcontainers-go` | unitários e integração com Postgres real |
| Local | `docker-compose` (API + Postgres) | |
| Docs da API | OpenAPI 3 (`api/openapi.yaml`) | requisito não funcional 6 |

## 2. Arquitetura

Três camadas por módulo de domínio:

```
HTTP handler  ->  service (regras de negócio)  ->  repository (SQL via sqlc)
```

- **handler:** recebe `*gin.Context`, faz o binding e a validação de formato, chama o service e traduz erros de domínio em status HTTP. Não tem regra de negócio. O `*gin.Context` não passa do handler: o service recebe `context.Context` e tipos de domínio, o que mantém as regras independentes do framework.
- **service:** contém as regras (parcelas, vencimento, painel). Depende de interfaces de repositório e de um `Clock`, por isso é testável sem banco.
- **repository:** única camada que conhece SQL. Toda função recebe `userID` e filtra por ele (isolamento, R1.5).

### Estrutura de pastas
```
finance-app/
  cmd/api/main.go
  api/openapi.yaml
  internal/
    platform/
      config/        leitura de env
      database/      pool pgx, transações
      httpx/         router Gin, middlewares (auth, logging, recovery), respostas JSON, erros
      clock/         interface Clock (permite fixar "hoje" em testes)
    money/           tipo Cents, Split, formatação
    auth/            registro, login, JWT, middleware
    income/
    category/
    room/
    recurring/       contas recorrentes e pagamentos mensais
    expense/         gastos e parcelas
    dashboard/       painel mensal, projeção, resumo da casa
  db/
    migrations/
    queries/         arquivos .sql do sqlc
  docker-compose.yml
  sqlc.yaml
  Makefile
```

Um módulo não importa o repositório de outro. O `dashboard` consulta o banco com suas próprias queries de leitura, pois é um caso de agregação entre tabelas.

## 3. Modelo de dados

Convenções: todo valor monetário é `BIGINT` em centavos. Todo registro de usuário tem `user_id`. Meses são guardados como `DATE` no primeiro dia do mês.

### Diagrama de relações
```
users 1 ─── N income
users 1 ─── N categories ─── N recurring_bills ─── N bill_payments
users 1 ─── N rooms
users 1 ─── N expenses  (N ─── 1 categories, N ─── 0..1 rooms)
expenses 1 ─── N installments   (somente crédito)
```

### Esquema

```sql
CREATE TABLE users (
  id            UUID PRIMARY KEY,
  email         TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_uq ON users (lower(email));

-- R2: renda com vigência a partir de um mês
CREATE TABLE income (
  id           UUID PRIMARY KEY,
  user_id      UUID NOT NULL REFERENCES users(id),
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
  valid_from   DATE NOT NULL CHECK (valid_from = date_trunc('month', valid_from)::date),
  UNIQUE (user_id, valid_from)
);

-- R3
CREATE TABLE categories (
  id      UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  name    TEXT NOT NULL
);
CREATE UNIQUE INDEX categories_name_uq ON categories (user_id, lower(name));

-- R7
CREATE TABLE rooms (
  id      UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  name    TEXT NOT NULL
);
CREATE UNIQUE INDEX rooms_name_uq ON rooms (user_id, lower(name));

-- R4: cada edição cria uma nova versão; a anterior é encerrada
CREATE TABLE recurring_bills (
  id            UUID PRIMARY KEY,
  user_id       UUID NOT NULL REFERENCES users(id),
  category_id   UUID NOT NULL REFERENCES categories(id),
  name          TEXT NOT NULL,
  amount_cents  BIGINT NOT NULL CHECK (amount_cents > 0),
  is_estimate   BOOLEAN NOT NULL DEFAULT false,
  due_day       SMALLINT NOT NULL CHECK (due_day BETWEEN 1 AND 31),
  active_from   DATE NOT NULL,          -- primeiro dia do mês
  active_until  DATE,                   -- primeiro dia do último mês ativo; NULL = sem fim
  CHECK (active_until IS NULL OR active_until >= active_from)
);

-- R5: ausência de linha significa "pendente"
CREATE TABLE bill_payments (
  id                 UUID PRIMARY KEY,
  user_id            UUID NOT NULL REFERENCES users(id),
  recurring_bill_id  UUID NOT NULL REFERENCES recurring_bills(id),
  month              DATE NOT NULL,
  paid_amount_cents  BIGINT NOT NULL CHECK (paid_amount_cents > 0),
  paid_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (recurring_bill_id, month)
);

-- R6, R7
CREATE TABLE expenses (
  id             UUID PRIMARY KEY,
  user_id        UUID NOT NULL REFERENCES users(id),
  category_id    UUID NOT NULL REFERENCES categories(id),
  room_id        UUID REFERENCES rooms(id),       -- NOT NULL = compra da casa
  description    TEXT NOT NULL,                   -- texto livre, pode citar a loja
  amount_cents   BIGINT NOT NULL CHECK (amount_cents > 0),
  spent_at       DATE NOT NULL,
  payment_method TEXT NOT NULL CHECK (payment_method IN ('pix','debit','credit','cash','boleto')),
  installments_n SMALLINT CHECK (installments_n BETWEEN 1 AND 48),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- parcelas só existem para crédito
  CHECK ((payment_method = 'credit') = (installments_n IS NOT NULL))
);
CREATE INDEX expenses_user_date_idx ON expenses (user_id, spent_at);

-- Somente para gastos no crédito
CREATE TABLE installments (
  id           UUID PRIMARY KEY,
  expense_id   UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
  number       SMALLINT NOT NULL CHECK (number >= 1),
  due_month    DATE NOT NULL,           -- primeiro dia do mês da fatura
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
  UNIQUE (expense_id, number)
);
CREATE INDEX installments_due_idx ON installments (due_month);
```

`installments` não tem `user_id`. O isolamento vem do join com `expenses`. As queries sempre passam por `expenses.user_id`.

### Decisões de modelagem
- **Parcelas só no crédito (R6.4):** o `CHECK` em `expenses` impede `installments_n` em gasto que não seja crédito e obriga o campo quando for.
- **Compra da casa (R7.1):** não há coluna de tipo. É compra da casa quem tem `room_id IS NOT NULL`.
- **Loja:** não existe campo próprio. Vai em `description`.
- **Versionamento de conta recorrente (R4.5, R4.6):** editar fecha a versão atual (`active_until` = mês anterior) e cria outra com `active_from` = mês corrente. Desativar só define `active_until`. Meses passados continuam apontando para a versão que valia.
- **Pendente é derivado (R5.1):** não se gera linha para cada mês. Uma conta é pendente se estiver ativa no mês e não existir `bill_payments` para ela naquele mês. Isso permite consultar meses futuros sem tarefa agendada.
- **Renda com vigência (R2.3):** a renda de um mês é a linha com maior `valid_from` menor ou igual ao mês.
- **Categoria e ambiente em uso (R3.3, R7.9):** `ON DELETE` padrão (`RESTRICT`). O service captura a violação de chave estrangeira e devolve erro de domínio.

## 4. Regras de domínio

### 4.1 Dinheiro (`internal/money`)
```go
type Cents int64

// Split divide total em n parcelas. O resto vai para a primeira (R6.6).
func Split(total Cents, n int) ([]Cents, error)
```
Exemplo: `Split(100, 3)` retorna `[34, 33, 33]`. Erro se `n < 1`, `n > 48` ou `total <= 0`.

### 4.2 Datas
- **Mês da primeira fatura (R6.4):** é o mês de `spent_at`. O MVP não modela data de fechamento da fatura do cartão. Premissa registrada: se a compra for feita depois do fechamento, o usuário ajusta a data.
- **Parcela k:** `due_month = addMonths(firstMonth, k-1)`.
- **Dia de vencimento (R4.4):** `effectiveDay = min(due_day, lastDayOfMonth)`.
- **Hoje:** vem de `Clock`, nunca de `time.Now()` direto.

### 4.3 Painel mensal (R8)
Para o mês `M` do usuário `U`:

| Parte | Fonte |
|---|---|
| Contas recorrentes (previsto) | `recurring_bills` com `active_from <= M` e (`active_until IS NULL` ou `active_until >= M`) |
| Contas recorrentes (pago) | `bill_payments` de `M` |
| Gastos à vista | `expenses` não crédito com `spent_at` dentro de `M` |
| Parcelas | `installments` com `due_month = M`, via join com `expenses` do usuário |
| Compras da casa (subtotal) | gastos à vista e parcelas do mês cujo `expenses.room_id IS NOT NULL` |

- **Custo de vida base (R8.5):** soma do valor previsto das contas recorrentes ativas.
- **Total do mês (R8.1):** para cada conta recorrente, vale o valor pago, se houver pagamento; senão, o previsto. Somam-se gastos à vista e parcelas.
- **Por categoria (R8.2):** as três fontes agrupadas por `category_id`. Parcela herda a categoria do gasto de origem.
- **Renda (R8.3, R8.4):** `% comprometido = total * 10000 / renda` (inteiro, em pontos-base, ou seja, 2.534 = 25,34%). Saldo = renda - total. Sem renda, os dois campos são omitidos.
- **Mês futuro (R8.6):** a mesma consulta funciona. Não haverá gastos à vista futuros, e as parcelas já existem.

### 4.4 Projeção (R9)
Para cada um dos 12 meses seguintes ao atual: soma do previsto das contas recorrentes ativas mais as parcelas com `due_month` naquele mês. O total de parcelas a pagar é a soma de `installments.amount_cents` com `due_month` posterior ao mês atual.

### 4.5 Resumo da casa (R7.5)
Por ambiente:
- **Pago:** gastos à vista com `spent_at <= hoje` mais parcelas com `due_month <= mês atual`.
- **A pagar:** parcelas com `due_month > mês atual`.

### 4.6 Edição de gasto (R6.12)
Em uma transação: atualiza o gasto, apaga as parcelas antigas e gera as novas com `Split`. A soma das parcelas é verificada antes do commit.

### 4.7 Edição de conta recorrente (R4.5)
Em uma transação: encerra a versão atual, cria a nova versão e, se houver pagamento no mês corrente, aponta esse pagamento para a nova versão. Assim o pagamento já feito não fica órfão.

## 5. API

Prefixo `/v1`. JSON em todas as respostas. Rotas exceto `/auth/*` exigem `Authorization: Bearer <token>`.

| Método e rota | Requisito |
|---|---|
| `POST /auth/register` | R1.1, R1.2 |
| `POST /auth/login` | R1.3, R1.4 |
| `PUT /income` (corpo: `amount_cents`, `valid_from`) | R2 |
| `GET /income?month=AAAA-MM` | R2 |
| `GET /categories`, `POST /categories`, `DELETE /categories/{id}` | R3 |
| `GET /rooms`, `POST /rooms`, `DELETE /rooms/{id}` | R7.6 a R7.10 |
| `GET /recurring-bills`, `POST /recurring-bills` | R4.1, R4.7 |
| `PUT /recurring-bills/{id}` | R4.5 |
| `DELETE /recurring-bills/{id}` (desativa) | R4.6 |
| `PUT /recurring-bills/{id}/payments/{month}` (corpo: `paid_amount_cents` opcional) | R5.2, R5.3 |
| `DELETE /recurring-bills/{id}/payments/{month}` | R5.4 |
| `POST /expenses` | R6.1 a R6.7, R7.1 |
| `GET /expenses?month=AAAA-MM&room_id=` | R6.8, R7.4 |
| `GET /expenses/{id}` (inclui parcelas) | R6.9 |
| `PUT /expenses/{id}`, `DELETE /expenses/{id}` | R6.10 a R6.12 |
| `GET /dashboard?month=AAAA-MM` | R8 |
| `GET /projection` | R9 |
| `GET /rooms/summary` | R7.5 |

### Exemplo: criar gasto parcelado
```http
POST /v1/expenses
{
  "description": "Sofá 3 lugares, Tok&Stok",
  "amount_cents": 320000,
  "category_id": "…",
  "room_id": "…",
  "spent_at": "2026-12-05",
  "payment_method": "credit",
  "installments_n": 10
}
```
Resposta `201`: o gasto e as 10 parcelas de R$ 320,00, com `due_month` de `2026-12-01` a `2027-09-01`.

### Erros
Formato único, estilo `application/problem+json`:
```json
{ "code": "category_in_use", "message": "…" }
```

| Situação | Status |
|---|---|
| Corpo inválido ou regra violada (valor ≤ 0, parcelas fora de 1 a 48, dia de vencimento inválido) | 400 |
| Credenciais inválidas ou token ausente/expirado | 401 |
| Recurso inexistente ou de outro usuário | 404 (não revela existência) |
| E-mail, categoria ou ambiente duplicado | 409 |
| Exclusão de categoria ou ambiente em uso | 409 |

## 6. Segurança
- Senha com bcrypt (custo 12). Login devolve o mesmo erro para e-mail ou senha incorretos (R1.4).
- JWT HS256, expiração de 24 h, segredo em `JWT_SECRET`. O `user_id` vem do token e é passado a todo repository.
- Referências cruzadas (categoria, ambiente) são validadas pelo `user_id` antes de gravar (R7.10).
- Todo SQL é parametrizado (garantido pelo `sqlc`).
- Cadastro cria usuário, categorias e ambientes padrão na mesma transação (R3.1, R7.6).

## 7. Estratégia de testes

| Camada | O que cobre | Ferramenta |
|---|---|---|
| Unitário | `money.Split` (resto, n = 1, limites), data efetiva de vencimento (31 em fevereiro, ano bissexto), cálculo de `addMonths`, percentual comprometido | `testing`, tabela de casos |
| Service | geração de parcelas, edição de gasto, versionamento de conta recorrente, painel, com repositórios falsos e `Clock` fixo | `testing` |
| Integração | repositórios e queries de agregação contra Postgres real; isolamento entre usuários; violações de `CHECK` e chave estrangeira | `testcontainers-go` |
| API | fluxo completo: cadastro, login, conta recorrente, gasto parcelado, painel | `httptest` |

Teste de propriedade para o `Split`: para qualquer `total` e `n` válidos, a soma das parcelas é igual ao total e nenhuma parcela é menor que 1.

## 8. Premissas e pontos em aberto
1. **Fechamento da fatura:** não modelado. A primeira parcela cai no mês de `spent_at`.
2. **Moeda única:** BRL.
3. **Juros:** o valor total informado já inclui juros de parcelamento, se houver.
4. **Total do mês usa o pago quando existe:** a conta paga por valor diferente do previsto altera o total real, mas não o custo de vida base.
5. **Paginação:** fora do MVP. A listagem é sempre filtrada por mês.
6. **Fuso horário:** datas de gasto são `DATE` sem fuso. "Hoje" usa `America/Sao_Paulo`.
