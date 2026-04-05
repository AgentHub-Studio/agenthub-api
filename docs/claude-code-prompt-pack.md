# Pacote de prompts inspirado no Claude Code

Este pacote adiciona templates globais em `prompt_template` com conteúdo adaptado do repositório público `claude-code`.

Importante:

- O prompt-base proprietário do Claude Code não está exposto no repositório público.
- Portanto, este pacote **não tenta clonar o Claude Code inteiro**.
- Ele empacota apenas comportamentos que estão publicamente documentados ou explicitamente codificados em plugins, referências e snippets.

## Templates adicionados

| Nome | Slug | Uso recomendado |
|---|---|---|
| Claude Code Explanatory Mode | `claude-code-explanatory-mode` | Estilo de saída mais didático |
| Claude Code Learning Mode | `claude-code-learning-mode` | Sessões colaborativas em que o usuário escreve partes pequenas e significativas |
| Claude Code Anti-Overengineering Guardrails | `claude-code-anti-overengineering-guardrails` | Guardrails para evitar abstrações e complexidade desnecessárias |
| Claude Code Code Exploration Guardrails | `claude-code-code-exploration-guardrails` | Obrigar leitura real do código antes de propor mudanças |
| Claude Code Frontend Aesthetics | `claude-code-frontend-aesthetics` | Melhorar qualidade visual de prompts para frontend |
| Claude Code Agent Creator | `claude-code-agent-creator` | Base para criação de agentes/skills via LLM |

## Como usar no AgentHub

### 1. Como template reutilizável para agentes

Use esses registros como ponto de partida ao criar ou atualizar `system_prompt` de agentes no painel/API.

Recomendação:

- `claude-code-anti-overengineering-guardrails`
- `claude-code-code-exploration-guardrails`

Esses dois são os mais úteis como blocos permanentes em agentes de desenvolvimento.

### 2. Como conteúdo para slugs dinâmicos do runtime agentic

Os slugs abaixo já são resolvidos pelo runtime do `agenthub-api`:

- `agentic-user-interaction-policy`
- `agentic-tool-usage-instructions`
- `agentic-tool-use-summary-system-prompt`
- `agentic-session-memory-extraction-prompt`

Uso recomendado:

- incorporar `claude-code-anti-overengineering-guardrails` e `claude-code-code-exploration-guardrails` dentro de `agentic-tool-usage-instructions`
- incorporar `claude-code-explanatory-mode` ou `claude-code-learning-mode` em agentes específicos, não globalmente para todos os tenants, por causa de custo e mudança de estilo
- usar `claude-code-frontend-aesthetics` em prompts de agentes voltados a frontend

### 3. Como base para criação de novos agentes

`claude-code-agent-creator` é melhor usado em fluxos de criação guiada de agentes/skills, não no prompt geral do chat.

## O que não deve ser copiado literalmente

As partes abaixo dependem de arquitetura própria do Claude Code e precisam de adaptação:

- hooks `SessionStart`, `Stop`, `PreToolUse` e afins
- semântica de subagents do produto original
- convenções de `CLAUDE.md`
- UI do terminal e affordances do cliente CLI

## Fontes públicas usadas

- `plugins/explanatory-output-style/`
- `plugins/learning-output-style/`
- `plugins/claude-opus-4-5-migration/.../prompt-snippets.md`
- `plugins/plugin-dev/skills/agent-development/references/agent-creation-system-prompt.md`
- `plugins/plugin-dev/skills/agent-development/references/system-prompt-design.md`
