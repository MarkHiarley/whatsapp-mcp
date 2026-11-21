# DESIGN.md — Painel WhatsApp

> Direção inferida do brief; substituível pela identidade do cliente. Mundo: **sala de sinal**.

## Modo

Operate. Superfície de trabalho densa, previsível, com um único momento dimensional: o **orb de sinal** que codifica o estado da sessão. Nada de 3D atrás de tabelas.

## Paleta

- Base grafite quase neutra (dark-first), uma segunda camada para sidebar/toolbars.
- Um único acento: verde WhatsApp (`--primary`), usado em ação primária, seleção e estado conectado.
- Semântica: conectado verde, pareando âmbar, desconectado cinza, deslogado vermelho. Nunca usar o acento como decoração.
- Texto secundário tintado da superfície, nunca cinza puro sobre cor.

## Tipografia

- Uma família: Geist (já instalada). Sem display face em labels/botões/dados.
- Escala fixa em rem, razão 1.125–1.2. Números tabulares em métricas.
- Prosa 65–75ch; tabelas podem ser densas.

## Layout

- Sidebar com grupos (Operação, Eventos, Configuração), tenant atual e pill de status.
- Overview: faixa de status com o orb + JID + última atividade; linha compacta de métricas (definition list, não quatro cards iguais); tabela de atividade recente real; ações rápidas.
- Tabelas densas, cabeçalho sticky, scroll horizontal controlado no mobile.
- Breakpoint estrutural: sidebar colapsa; nada de tipografia fluida.

## Movimento

- 150–250ms em transições; ease-out exponencial.
- O orb é o único movimento autoral: gira devagar e pulsa só quando `connected`; estático com `prefers-reduced-motion`; pausa com a aba oculta.
- Sem sequência de entrada orquestrada, sem parallax, sem glass/gradiente decorativo.

## Componentes

- Estados default/hover/focus/active/disabled/loading/error em todo controle.
- Skeletons no carregamento; empty states que ensinam a próxima ação.
- Dialogs para criação de key/webhook/tenant (token mostrado uma vez); confirmação para revogar/remover.
- Foco visível, seleção de texto, scrollbar e caret tematizados.
- Ícones `lucide-react`, um peso; sem emoji como ícone.

## Recusas

- Gradient text, glass, sombra dura, kicker/eyebrow, números de seção decorativos.
- Cards iguais de ícone+título+texto como estrutura; hero-metric (número grande + label + stats).
- Monospace como fantasia; usar só para dados/medições.

## Referências para refactor futuro

- https://modulify.ai/templates — analisar depois os padrões de templates/blocks (composição de seções, densidade, empty states e blocos de dados) para uma passada de refactor do painel.

