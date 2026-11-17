# Plano de implementação: MVP de Controle Financeiro

Stack: Go, Gin, PostgreSQL, pgx, sqlc, goose. Cada tarefa referencia os requisitos de `requirements.md` e as seções de `design.md`. As tarefas são ordenadas para que cada uma deixe o projeto compilando e testável.

Convenção: toda tarefa de regra de negócio inclui os testes unitários da própria regra.

## Fase 0. Fundação

- [ ] 1. Inicializar o projeto
  - Criar o módulo Go (`finance-app`), a estrutura de pastas do design (seção 2) e o `Makefile` (alvos: `run`, `test`, `migrate-up`, `migrate-down`, `sqlc`).
  - Criar `docker-compose.yml` com Postgres 16 e a API, e `.env.example` (`DATABASE_URL`, `JWT_SECRET`, `PORT`).
  - Criar `internal/platform/config` lendo variáveis de ambiente, com erro claro se faltar alguma obrigatória.
  - _Requisitos: NFR 2_

- [ ] 2. Servidor Gin base
  - Criar `cmd/api/main.go` com Gin, graceful shutdown e o endpoint `GET /healthz`.
  - Criar `internal/platform/httpx` com middlewares de recovery e logging, e o formato único de erro (`code`, `message`) do design (seção 5).
  - _Requisitos: NFR 6_

- [ ] 3. Banco, migrations e sqlc
  - Criar pool `pgx` em `internal/platform/database`, com helper para executar uma função dentro de transação.
  - Escrever as migrations do goose com o esquema do design (seção 3): `users`, `income`, `categories`, `rooms`, `recurring_bills`, `bill_payments`, `expenses`, `installments`.
  - Configurar `sqlc.yaml` e gerar o código a partir de um primeiro arquivo de queries.
  - _Requisitos: NFR 1, NFR 3_

- [ ] 4. Pacotes utilitários de domínio
  - 4.1 `internal/clock`: interface `Clock`, implementação real (fuso `America/Sao_Paulo`) e implementação fixa para testes.
  - 4.2 `internal/money`: tipo `Cents` e `Split(total, n)` com o resto na primeira parcela. Testes em tabela e teste de propriedade (soma igual ao total, nenhuma parcela menor que 1).
  - 4.3 Funções de data: `addMonths`, `firstOfMonth` e `effectiveDueDay` (dia 31 em fevereiro, ano bissexto). Testes em tabela.
  - _Requisitos: R4.4, R6.6, R6.7, NFR 1, NFR 5_

## Fase 1. Acesso e cadastros básicos

- [ ] 5. Autenticação
  - Criar `auth`: `POST /v1/auth/register` e `POST /v1/auth/login`, com bcrypt e JWT (24 h).
  - Middleware que valida o token e coloca o `user_id` no contexto.
  - Registro cria usuário, categorias padrão e ambientes padrão na mesma transação.
  - Login devolve o mesmo erro para e-mail ou senha incorretos.
  - Testes: e-mail duplicado (409), credenciais inválidas (401), token ausente ou expirado (401).
  - _Requisitos: R1.1 a R1.5, R3.1, R7.6_

- [ ] 6. Categorias
  - `GET /v1/categories`, `POST /v1/categories`, `DELETE /v1/categories/{id}`.
  - Nome único por usuário sem diferenciar maiúsculas; exclusão de categoria em uso devolve 409.
  - Testes de integração, incluindo isolamento entre dois usuários.
  - _Requisitos: R3.2, R3.3, R1.5_

- [ ] 7. Ambientes (rooms)
  - `GET /v1/rooms`, `POST /v1/rooms`, `DELETE /v1/rooms/{id}`.
  - Nome único por usuário sem diferenciar maiúsculas; exclusão de ambiente com gastos devolve 409.
  - _Requisitos: R7.6 a R7.9_

- [ ] 8. Renda
  - `PUT /v1/income` (`amount_cents`, `valid_from`) e `GET /v1/income?month=AAAA-MM`.
  - A renda de um mês é a linha com maior `valid_from` menor ou igual ao mês. Valor menor ou igual a zero devolve 400.
  - Testes: alteração de renda não muda meses anteriores.
  - _Requisitos: R2.1 a R2.3_

- [ ] 9. Checkpoint da Fase 1
  - Rodar todos os testes. Subir o ambiente com `docker-compose` e validar à mão: cadastro, login, criação de categoria e ambiente. Perguntar ao usuário se há dúvidas.

## Fase 2. Custo de vida (núcleo do produto)

- [ ] 10. Contas recorrentes
  - `GET /v1/recurring-bills?month=`, `POST /v1/recurring-bills`, `PUT /v1/recurring-bills/{id}`, `DELETE /v1/recurring-bills/{id}`.
  - Validar `due_day` entre 1 e 31 e valor maior que zero. Categoria deve pertencer ao usuário.
  - Edição versionada: encerra a versão atual e cria a nova a partir do mês corrente, em transação (design 4.7). Desativar define `active_until`.
  - A listagem mostra o total mensal das contas ativas.
  - Testes de service com `Clock` fixo: edição preserva o histórico; desativar mantém meses passados.
  - _Requisitos: R4.1 a R4.7_

- [ ] 11. Pagamento das contas do mês
  - `PUT /v1/recurring-bills/{id}/payments/{month}` (valor real opcional, padrão igual ao previsto) e `DELETE` do mesmo caminho.
  - Pendente é derivado: sem linha em `bill_payments` significa pendente.
  - Ao editar uma conta com pagamento no mês corrente, o pagamento é apontado para a nova versão (testar).
  - _Requisitos: R5.1 a R5.4_

## Fase 3. Gastos e compras da casa

- [ ] 12. Criação de gastos e parcelas
  - `POST /v1/expenses`: valida valor, data, categoria e ambiente (ambos do usuário), e `installments_n` entre 1 e 48.
  - Crédito sem parcelas informadas assume 1. Crédito gera N parcelas com `Split` e `due_month` a partir do mês de `spent_at`, em transação.
  - Formas à vista não geram parcelas.
  - Testes: 3.200,00 em 10x; 100 centavos em 3x (34, 33, 33); limites 0, 1, 48 e 49; ambiente de outro usuário (rejeitar).
  - _Requisitos: R6.1 a R6.7, R7.1, R7.2, R7.10_

- [ ] 13. Consulta, edição e exclusão de gastos
  - `GET /v1/expenses?month=&room_id=` ordenado por data decrescente, com filtro opcional por ambiente.
  - `GET /v1/expenses/{id}` com as parcelas e a quantidade de vencidas.
  - `PUT /v1/expenses/{id}` regera as parcelas em transação; `DELETE` remove o gasto e suas parcelas (cascata).
  - Testes: a soma das parcelas continua igual ao novo valor após a edição.
  - _Requisitos: R6.8 a R6.12, R7.3, R7.4_

- [ ] 14. Resumo da casa
  - `GET /v1/rooms/summary`: por ambiente, total pago (à vista e parcelas vencidas) e total a pagar (parcelas futuras).
  - Testes com `Clock` fixo, cobrindo a fronteira do mês atual.
  - _Requisitos: R7.5_

- [ ] 15. Checkpoint da Fase 3
  - Rodar todos os testes. Validar à mão o cenário real: sofá em 10x no crédito, geladeira em 12x, cama no Pix, e conferir o resumo por ambiente. Perguntar ao usuário se há dúvidas.

## Fase 4. Painel e projeção

- [ ] 16. Painel mensal
  - `GET /v1/dashboard?month=AAAA-MM` com: custo de vida base, contas recorrentes (previsto e pago), gastos à vista, parcelas do mês, subtotal das compras da casa e total geral.
  - Total por categoria com percentual (parcela herda a categoria do gasto).
  - Percentual comprometido em pontos-base e saldo; omitidos quando não há renda.
  - Query de agregação com índice em `installments.due_month` e `expenses (user_id, spent_at)`.
  - Testes: mês passado, atual e futuro; com e sem renda; conta paga por valor diferente do previsto.
  - _Requisitos: R8.1 a R8.6, NFR 4_

- [ ] 17. Projeção
  - `GET /v1/projection`: próximos 12 meses (contas ativas previstas mais parcelas) e total de parcelas ainda a pagar.
  - _Requisitos: R9.1, R9.2_

## Fase 5. Qualidade e entrega

- [ ] 18. Teste de ponta a ponta da API
  - Com `httptest` e Postgres via `testcontainers-go`: cadastro, login, renda, contas recorrentes, gasto parcelado, painel e projeção em um fluxo único.
  - Teste de isolamento: o usuário B nunca vê nem altera dados do usuário A em nenhuma rota.
  - _Requisitos: R1.5, NFR 3_

- [ ] 19. Documentação
  - `api/openapi.yaml` cobrindo todas as rotas, exemplos e códigos de erro.
  - `README.md`: objetivo, como subir com `docker-compose`, como rodar testes, decisões de arquitetura e premissas (design seção 8).
  - _Requisitos: NFR 6_

- [ ] 20. Checkpoint final
  - Rodar todos os testes e conferir cada critério de aceite de `requirements.md` contra a implementação. Listar o que ficou de fora.

## Etapa 2 (fora deste plano)
- Lançamento de gasto em linguagem natural com LLM.
- Resumo mensal em texto sobre a saúde financeira.
- Interface web.
- Conta compartilhada entre casal.
- Data de fechamento da fatura do cartão.
