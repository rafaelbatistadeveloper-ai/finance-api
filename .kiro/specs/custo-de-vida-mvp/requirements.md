# Requisitos: MVP de Controle Financeiro (Custo de Vida)

## Introdução

Aplicativo SaaS para entender a saúde financeira pessoal. O usuário informa as contas recorrentes do mês para obter seu custo de vida base. Depois acrescenta gastos variáveis e as compras da casa nova, com forma de pagamento e parcelamento.

Convenções do documento:
- Valores monetários em BRL, armazenados em centavos.
- Mês de referência no formato `AAAA-MM`.
- Critérios de aceite em notação EARS (QUANDO / SE / ENTÃO / O SISTEMA DEVE).

## Fora do escopo do MVP
- Integração bancária (Open Finance) e importação de extrato.
- Conta compartilhada entre várias pessoas (casal).
- Metas, alertas e notificações.
- Lançamento por linguagem natural com IA (candidato à etapa 2).

---

## Requisitos

### R1. Cadastro e acesso
**História:** Como usuário, quero criar uma conta e entrar nela, para que meus dados financeiros fiquem privados.

**Critérios de aceite**
1. QUANDO o usuário informar e-mail e senha válidos no cadastro, ENTÃO o sistema DEVE criar a conta e permitir o login.
2. SE o e-mail já estiver cadastrado, ENTÃO o sistema DEVE rejeitar o cadastro com mensagem de erro.
3. QUANDO o usuário informar credenciais corretas no login, ENTÃO o sistema DEVE emitir um token de acesso com expiração.
4. SE as credenciais forem inválidas, ENTÃO o sistema DEVE negar o acesso sem revelar qual campo está errado.
5. O sistema DEVE garantir que cada usuário acesse apenas os seus próprios dados.

### R2. Renda mensal
**História:** Como usuário, quero informar minha renda mensal, para saber quanto dela está comprometida.

**Critérios de aceite**
1. QUANDO o usuário informar um valor de renda, ENTÃO o sistema DEVE salvá-lo como renda mensal atual.
2. SE o valor informado for menor ou igual a zero, ENTÃO o sistema DEVE rejeitá-lo.
3. QUANDO a renda for alterada, ENTÃO o sistema DEVE usar o novo valor nos meses a partir da alteração, sem mudar meses anteriores.

### R3. Categorias
**História:** Como usuário, quero classificar meus gastos em categorias, para entender para onde o dinheiro está indo.

**Critérios de aceite**
1. QUANDO o usuário criar a conta, ENTÃO o sistema DEVE disponibilizar categorias padrão (moradia, energia, internet, alimentação, transporte, lazer, saúde, casa/mobília, outros).
2. QUANDO o usuário criar uma categoria nova com nome único, ENTÃO o sistema DEVE salvá-la.
3. SE o usuário tentar excluir uma categoria em uso, ENTÃO o sistema DEVE impedir a exclusão e informar o motivo.

### R4. Contas recorrentes
**História:** Como usuário, quero listar minhas contas recorrentes (internet, energia, financiamento), para ter a base do meu custo de vida mensal.

**Critérios de aceite**
1. QUANDO o usuário cadastrar uma conta recorrente com nome, categoria, valor e dia de vencimento, ENTÃO o sistema DEVE salvá-la como ativa.
2. O sistema DEVE permitir marcar o valor como "média estimada", para contas que variam (ex.: energia).
3. SE o dia de vencimento não estiver entre 1 e 31, ENTÃO o sistema DEVE rejeitar o cadastro.
4. QUANDO o dia de vencimento não existir no mês (ex.: 31 em fevereiro), ENTÃO o sistema DEVE considerar o último dia do mês.
5. QUANDO o usuário editar uma conta recorrente, ENTÃO o sistema DEVE aplicar a mudança apenas a partir do mês corrente, preservando o histórico.
6. QUANDO o usuário desativar uma conta recorrente, ENTÃO o sistema DEVE deixar de incluí-la nos meses futuros e mantê-la nos meses passados.
7. QUANDO o usuário listar as contas, ENTÃO o sistema DEVE exibir o total mensal das contas ativas.

### R5. Pagamento das contas do mês
**História:** Como usuário, quero registrar o valor real pago de cada conta recorrente, para comparar o previsto com o realizado.

**Critérios de aceite**
1. QUANDO um mês começar, ENTÃO o sistema DEVE considerar cada conta recorrente ativa como pendente, com o valor previsto.
2. QUANDO o usuário marcar uma conta como paga e informar o valor real, ENTÃO o sistema DEVE registrar o pagamento naquele mês.
3. SE o usuário não informar o valor real, ENTÃO o sistema DEVE usar o valor previsto.
4. QUANDO o usuário desfizer um pagamento, ENTÃO o sistema DEVE voltar a conta ao estado pendente.

### R6. Gastos (variáveis e parcelados)
**História:** Como usuário, quero lançar gastos, com a forma de pagamento e o parcelamento, para saber quanto gasto além das contas fixas e quanto ainda vou pagar.

**Critérios de aceite**
1. QUANDO o usuário lançar um gasto com descrição, valor total, categoria, data e forma de pagamento (Pix, débito, crédito, dinheiro, boleto), ENTÃO o sistema DEVE salvá-lo.
2. SE o valor for menor ou igual a zero ou a data for inválida, ENTÃO o sistema DEVE rejeitar o lançamento.
3. QUANDO a forma de pagamento for Pix, débito, dinheiro ou boleto, ENTÃO o sistema DEVE lançar o valor total no mês da data do gasto.
4. QUANDO a forma de pagamento for crédito com N parcelas (N ≥ 1), ENTÃO o sistema DEVE gerar N parcelas mensais, uma por mês, a partir do mês da primeira fatura.
5. SE a forma de pagamento for crédito e o número de parcelas não for informado, ENTÃO o sistema DEVE assumir 1 parcela.
6. O sistema DEVE dividir o valor total em parcelas de centavos inteiros, e a soma das parcelas DEVE ser igual ao valor total (a diferença de centavos vai para a primeira parcela).
7. SE o número de parcelas for menor que 1 ou maior que 48, ENTÃO o sistema DEVE rejeitar o lançamento.
8. QUANDO o usuário listar gastos de um mês, ENTÃO o sistema DEVE retornar os lançamentos ordenados por data, do mais recente ao mais antigo.
9. QUANDO o usuário consultar um gasto parcelado, ENTÃO o sistema DEVE exibir as parcelas e quantas já venceram.
10. O sistema DEVE permitir editar e excluir um gasto lançado.
11. QUANDO o usuário excluir um gasto parcelado, ENTÃO o sistema DEVE remover também todas as parcelas associadas.
12. QUANDO o usuário alterar o valor, a forma de pagamento ou o número de parcelas de um gasto, ENTÃO o sistema DEVE regerar as parcelas, mantendo a soma igual ao novo valor total.

### R7. Compras da casa nova e ambientes
**História:** Como usuário, quero associar gastos aos ambientes da casa nova, para controlar quanto já gastei e quanto ainda vou pagar em cada cômodo.

**Regra de modelagem:** um gasto (R6) com ambiente associado é uma compra da casa. Um gasto sem ambiente é um gasto comum.

**Critérios de aceite**
1. QUANDO o usuário lançar um gasto informando um ambiente (um dos ambientes cadastrados do usuário), ENTÃO o sistema DEVE tratá-lo como compra da casa.
2. O ambiente DEVE ser opcional; SE não for informado, ENTÃO o gasto é um gasto comum.
3. O sistema DEVE permitir que a descrição do gasto inclua detalhes livres, como a loja onde a compra foi feita. Loja não é uma entidade nem um campo separado.
4. QUANDO o usuário listar gastos, ENTÃO o sistema DEVE permitir filtrar por ambiente.
5. QUANDO o usuário consultar o resumo da casa, ENTÃO o sistema DEVE exibir, por ambiente, o total já pago (à vista e parcelas vencidas) e o total ainda a pagar (parcelas futuras).
6. QUANDO a conta for criada, ENTÃO o sistema DEVE disponibilizar ambientes padrão (sala, cozinha, quarto, banheiro, área de serviço, varanda, outros).
7. QUANDO o usuário criar um ambiente com nome único (sem diferenciar maiúsculas de minúsculas), ENTÃO o sistema DEVE salvá-lo.
8. SE o usuário tentar criar um ambiente com nome já existente, ENTÃO o sistema DEVE rejeitar o cadastro.
9. SE o usuário tentar excluir um ambiente que possui gastos, ENTÃO o sistema DEVE impedir a exclusão e informar o motivo.
10. SE o usuário informar um ambiente que não existe ou pertence a outro usuário, ENTÃO o sistema DEVE rejeitar o lançamento.

### R8. Painel mensal (custo de vida e saúde financeira)
**História:** Como usuário, quero ver um resumo do mês, para entender meu custo de vida e quanto da renda está comprometido.

**Critérios de aceite**
1. QUANDO o usuário consultar um mês, ENTÃO o sistema DEVE retornar: total de contas recorrentes (previsto e pago), total de gastos à vista, total de parcelas de gastos no crédito vencendo no mês e o total geral. O sistema DEVE informar também o subtotal das compras da casa (gastos com ambiente).
2. O sistema DEVE retornar o total do mês agrupado por categoria, com o percentual de cada uma.
3. QUANDO houver renda cadastrada, ENTÃO o sistema DEVE calcular o percentual da renda comprometido e o saldo restante (renda menos total geral).
4. SE não houver renda cadastrada, ENTÃO o sistema DEVE retornar os totais sem os campos de percentual e saldo.
5. O sistema DEVE retornar o custo de vida base, definido como a soma das contas recorrentes ativas no mês.
6. QUANDO o usuário consultar um mês futuro, ENTÃO o sistema DEVE incluir contas recorrentes previstas e parcelas já geradas, sem gastos variáveis.

### R9. Projeção de comprometimento
**História:** Como usuário, quero ver os próximos meses, para saber quanto já está comprometido por parcelas e contas fixas.

**Critérios de aceite**
1. QUANDO o usuário solicitar a projeção, ENTÃO o sistema DEVE retornar, para cada um dos próximos 12 meses, o total de contas recorrentes mais as parcelas devidas.
2. O sistema DEVE informar o total de parcelas ainda a pagar, somando todos os meses futuros.

---

## Requisitos não funcionais
1. **Precisão monetária:** nenhum cálculo pode usar ponto flutuante; todos os valores ficam em inteiros (centavos).
2. **Segurança:** senhas armazenadas com hash (bcrypt); segredos fora do código.
3. **Isolamento de dados:** toda consulta é filtrada pelo usuário autenticado.
4. **Desempenho:** o painel mensal responde em até 500 ms para até 5.000 lançamentos do usuário.
5. **Testabilidade:** regras de cálculo (parcelas, dia de vencimento, painel) cobertas por testes unitários.
6. **Documentação:** API descrita em OpenAPI.
