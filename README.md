<div align="center">
  <h1>Inox</h1>
  <p><strong>Controle explícito da CPU, com uma sintaxe mais legível.</strong></p>
  <p><code>v0.1</code> · <code>Go</code> · <code>x86-64</code> · <code>Linux</code></p>
</div>

---

Inox é uma linguagem experimental de baixo nível. Você escreve operações sobre registradores virtuais; o compilador valida o programa, escolhe registradores físicos e gera assembly NASM.

```inox
register a: u64 = 5
register b: u64 = 10 @rbx

add a, b
compare a, 15
jump_if equal, finished
sub a, 1

finished:
halt
```

## Comece

Requisitos: Go 1.23 ou superior. Para executar programas, instale também NASM e GNU `ld`. A opção `--vm` requer `qemu-x86_64` ou `qemu-x86_64-static` no `PATH`.

```sh
go build -o inoxc ./cmd/inoxc

# Gerar build/basic.asm
./inoxc build examples/basic.ix

# Executar o programa nativamente ou em QEMU user-mode
./inoxc run examples/basic.ix
./inoxc run examples/basic.ix --vm
```

O modo `--vm` executa o ELF Linux com o emulador x86-64 em modo usuário; ele não inicia uma máquina virtual completa.

## Comandos

| Comando | Ação |
| --- | --- |
| `inoxc build arquivo.ix` | Gera `build/arquivo.asm` em sintaxe NASM. |
| `inoxc build -o saida.asm arquivo.ix` | Escolhe o caminho do assembly gerado. |
| `inoxc inspect arquivo.ix --tokens` | Exibe os tokens. Também há `--ast`, `--ir` e `--asm`. |
| `inoxc run arquivo.ix` | Compila, monta, linka e executa nativamente. |
| `inoxc run arquivo.ix --vm` | Executa o ELF com `qemu-x86_64` ou `qemu-x86_64-static`. |

Para montar manualmente o arquivo gerado:

```sh
nasm -felf64 build/basic.asm -o build/basic.o
ld -o build/basic build/basic.o
./build/basic
```

## Escopo atual

- Tipos inteiros sem sinal: `u8`, `u16`, `u32` e `u64`.
- Instruções: `move`, `add`, `sub`, `compare`, `jump`, `jump_if` e `halt`.
- Até 14 registradores virtuais, alocados entre registradores gerais x86-64; o tipo seleciona o alias de largura (`rax`, `eax`, `ax`, `al`, por exemplo).
- Pinning opcional para controlar um registrador físico: `register result: u64 @rax`.
- Diagnósticos de lexer, parser e análise semântica com arquivo, linha e coluna.
- `halt` encerra o processo Linux com o syscall de saída.

`rsp` fica reservado para a stack, e `r11` para temporários do backend. A versão atual não tem strings, operações explícitas de memória, I/O da linguagem, biblioteca padrão ou imagem bootável para `qemu-system-x86_64`.

## Desenvolvimento

```sh
go test ./...
go vet ./...
```

O compilador não depende de bibliotecas Go externas. O código está dividido em lexer, parser, AST, análise semântica, IR, backend x86-64 e CLI.

## Exemplos

- [`examples/hello.ix`](examples/hello.ix): inicializa um registrador e encerra.
- [`examples/basic.ix`](examples/basic.ix): demonstra aritmética, comparação e desvio condicional.
