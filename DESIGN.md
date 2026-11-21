# DESIGN.md — Painel WhatsApp

Modo **Operate**: o usuário completa uma tarefa. Familiaridade, densidade e leitura rápida vencem expressão. A identidade vive em detalhes precisos, não em decoração.

## Direção

Painel claro, com sidebar branca, um único acento verde e um lima de seleção. Referências de composição: densidade de tabela do Sales CRM e tom de operação do Forge (galeria Modulify). Nada de roxo, glass, gradiente ou 3D atrás de dados.

## Tokens (claro = default)

- `--background #F6F6F3` · `--card`/`--sidebar #FFFFFF`
- `--foreground #171714` · `--muted-foreground #57574F`
- `--border`/`--input #E6E6DF`
- `--primary #167A3A` (`--primary-foreground #FFFFFF`) — ação primária e “conectado”
- `--highlight #E4F07A` (`--highlight-foreground #171714`) — item ativo e seleção
- `--banner #F5E38A` (`--banner-foreground #1A1908`) — só fora de produção
- `--destructive #C0362C`; pareando âmbar; desconectado cinza
- `--radius 1rem`: cards 16px, botões 12px, pílulas 999px; raio do filho ≤ raio do pai
- Sombra hairline: `0 1px 2px rgb(23 23 20 / 0.04), 0 1px 1px rgb(23 23 20 / 0.03)`
- Dark: `#0F0F0D`/cards `#171714`, mesmo verde, lima suavizado `#C9D96A`, nenhum resíduo roxo

## Tipografia

- Geist Sans em toda a UI. Geist Mono só em JID, key id, cursor e host.
- Sora no máximo no H1 do login.
- Escala 12/13/14/16/20/24/32; `tabular-nums` em números e datas; `pt-BR`.

## Layout

- Sidebar branca (~256px), borda clara, marca própria, `TenantCard` no topo, ícones Lucide finos, grupos Operação/Automação/Acesso/Sessão, ativo em lima com texto escuro, rodapé com avatar + nome/email + sair.
- Banner de ambiente só fora de produção; em produção o header começa no topo.
- Header do conteúdo: título à esquerda; `StatusBadge`, refresh e `ThemeToggle` à direita. Sem sino falso.
- Main em off-white, cards brancos, padding 16–24px, grid de 4px.
- Overview: 1–2 cards grandes (Mensagens, Eventos), faixa de métricas (Chats, Chaves, última atividade) e gráfico de 14 dias com `messages_by_day`. Só dados reais de `TenantStats`.

## Movimento

150–250ms, ease-out. `StatusOrb` é o único 3D e o único movimento autoral; com `prefers-reduced-motion` fica estático e pausa com a aba oculta.

## Componentes e estados

- Todo controle tem hover, focus-visible, active, disabled, loading e error.
- Empty states com título, motivo e CTA; skeletons no carregamento.
- Navegação com `<Link>`; `aria-current="page"`.
- Alvos ≥24px (44px no mobile); inputs mobile ≥16px.
- Status nunca só por cor: sempre com texto.

## Recusar

- Roxo, múltiplos acentos, verde neon em texto/botão, `#25D366` como texto.
- Sidebar escura no tema claro, glass/blur, gradient text, sombra forte, borda pesada.
- Métricas, percentuais, pílulas de período, sino, seletor ou “Modo Teste” inventados.
- Fonte display em label/botão/tabela; emoji no lugar de ícone; 3D atrás de dados; mock.
- Tabelas apertadas sem header sticky; empty em branco.

## Referências

- https://modulify.ai/templates — padrões de composição (Sales CRM, Forge).
- https://vercel.com/design/guidelines
- https://www.nngroup.com/articles/empty-state-interface-design/
- https://docs.stripe.com/stripe-apps/style
- https://www.saasui.design/blog/how-to-use-saas-ui-patterns-building-design-system
