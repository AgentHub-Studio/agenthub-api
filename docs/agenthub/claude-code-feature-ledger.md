# AgentHub API — Claude Code Feature Ledger

Registro das features extraídas do PDF arXiv:2604.14228v1 pelo arch loop.

---

## FEAT-015

| Campo | Valor |
|---|---|
| **Status** | DONE |
| **Iteração** | 125 |
| **Data** | 2026-05-11 |
| **Fonte** | arXiv:2604.14228v1 §2.3 "From Values to Architecture" + §2.4 "An Evaluative Lens: Long-term Capability Preservation" |

### Descrição

Implementação do `ValueArchitectureTraceRegistry` — mapeamento estruturado da §2.3 que conecta cada um dos cinco valores de design humano (Human Decision Authority, Safety, Reliable Execution, Capability Amplification, Contextual Adaptability) aos princípios de design que os operacionalizam e, em seguida, às decisões arquiteturais concretas que os implementam no Claude Code.

Inclui também o `EvaluativeLens` da §2.4 (Long-term Capability Preservation) como preocupação transversal que avalia todos os cinco valores sem ser um motor arquitetural primário. As ausências arquiteturais explícitas da §2.3 ("mappings also reveal what the architecture does *not* do") são formalizadas via `ArchitecturalAbsences()`.

### Critérios de Aceite

- [DONE] `ValueArchitectureTrace` struct: Value, MotivatedPrinciples, Decisions
- [DONE] `ArchitecturalDecision` struct: Label, Description, PrincipleID, ComponentRef, PDFSections
- [DONE] 5 traces completos (um por DesignValue) mapeados de §2.3
- [DONE] `TraceForValue(v)` retorna trace ou (zero, false)
- [DONE] `AllTraces()` em ordem canônica AllDesignValues
- [DONE] `DecisionsForPrinciple(id)` cruza os 5 traces
- [DONE] `DecisionsForValue(v)` atalho por valor
- [DONE] `EvaluativeLens` struct: ID, Label, Description, EvaluatedValues, PDFSection, EmpiricalBasis
- [DONE] `LensLongTermCapabilityPreservation` com evidência empírica (Huang et al., Shen e Tamkin)
- [DONE] `EvaluativeLenses()` e `LensForID(id)` no registry
- [DONE] `ArchitecturalAbsences()` retorna 3 ausências da §2.3
- [DONE] Todos os PrincipleID referenciados são válidos (sem dangling refs)
- [DONE] go vet limpo

### Implementação

| Arquivo | Conteúdo |
|---|---|
| `internal/domain/chat/agentic/value_architecture_trace.go` | Registry principal: tipos + 5 traces + EvaluativeLens + ArchitecturalAbsences |
| `internal/domain/chat/agentic/value_architecture_trace_test.go` | 24 testes unitários |
| `internal/domain/chat/agentic/value_architecture_trace_bdd_test.go` | 7 cenários BDD |

### Testes BDD

| Cenário | Resultado |
|---|---|
| `TestBDD_ValueArchitectureTrace_EachValueHasConcreteDecisions` (5 sub-testes) | PASS |
| `TestBDD_ValueArchitectureTrace_DenyFirstPrincipleIsSharedAcrossValues` | PASS |
| `TestBDD_ValueArchitectureTrace_ArchitecturalAbsencesAreFormallyRepresented` | PASS |
| `TestBDD_ValueArchitectureTrace_LongTermLensEvaluatesAllFiveValues` | PASS |
| `TestBDD_ValueArchitectureTrace_NoDanglingPrincipleReferences` | PASS |
| `TestBDD_ValueArchitectureTrace_CapabilityMotivatesThinHarnessDecision` | PASS |
| `TestBDD_ValueArchitectureTrace_AdaptabilityMotivatesFileBasedConfig` | PASS |

**Total: 24 unit + 7 BDD = 31 testes — todos PASS**
