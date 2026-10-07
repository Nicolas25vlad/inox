<div align="center">
  <h1>Inox</h1>
  <p><strong>Controle explícito da CPU, com sintaxe legível.</strong></p>
  <p><code>Go</code> · <code>x86-64 Linux</code> · <code>BIOS x86 16 bits</code></p>
</div>

---

Inox é uma linguagem experimental de baixo nível. O compilador tem dois alvos: programas ELF para Linux x86-64 e setores de boot BIOS em modo real. No alvo BIOS, você pode escrever NASM diretamente e usar pequenas operações prontas, como impressão por `INT 10h`.

## Comece

Requisitos: Go 1.23 ou superior e NASM. Para executar programas Linux, instale GNU `ld` e `qemu-x86_64`; para iniciar um programa BIOS, instale `qemu-system-x86_64`.

```sh
go build -o inoxc ./cmd/inoxc

# Linux x86-64: gera build/hello.asm
./inoxc build examples/hello.ix
./inoxc run examples/hello.ix --vm

# BIOS real mode: gera uma imagem de disquete e abre o QEMU
./inoxc build examples/boot.ix
./inoxc run examples/boot.ix --vm
```

Feche a janela do QEMU para encerrar a execução BIOS. No alvo Linux, `--vm` continua usando QEMU em modo usuário.

## Primeiro boot BIOS

```inox
target boot16

# Estágio 1: imprime pela interrupção BIOS INT 10h.
bios_print "Iniciando stage 1...\r\n"

# O compilador coloca este código no disco e o carrega via BIOS INT 13h.
stage2 {
    mov si, stage2_message
.print:
    lodsb
    test al, al
    jz .halt
    mov ah, 0x0e
    mov bx, 0x0007
    int 0x10
    jmp .print
.halt:
    cli
    hlt
    jmp .halt

stage2_message db 'Stage 2 carregado pela BIOS!', 0
}
```

`target boot16` gera uma imagem de disquete de 1,44 MiB. O primeiro setor contém o salto de entrada, segmentos e stack preparados, padding e a assinatura `55 AA` nos bytes 510–511. Quando existe `stage2`, o compilador lê seus setores com BIOS `INT 13h` e transfere a execução para `0x7e00`.

`bios_print` usa o serviço BIOS de teletype; use ASCII para manter o texto previsível entre BIOSes. O bloco `asm { ... }` aceita instruções, labels e diretivas NASM para controlar diretamente interrupções, portas, memória, flags e instruções da CPU. `stage2 { ... }` coloca código NASM após o setor de boot. O NASM valida esses blocos no build; o compilador não faz análise semântica das instruções dentro deles.

## Alvos e comandos

| Comando | Ação |
| --- | --- |
| `inoxc build arquivo.ix` | Gera NASM `.asm` para Linux ou imagem `.img` para `boot16`. |
| `inoxc build arquivo.ix -o saida` | Escolhe o caminho do artefato. |
| `inoxc inspect arquivo.ix --tokens` | Exibe os tokens; também há `--ast`, `--ir` e `--asm`. |
| `inoxc run arquivo.ix` | Monta, linka e executa o ELF Linux nativamente. |
| `inoxc run arquivo.ix --vm` | Executa Linux com `qemu-x86_64` ou inicia `boot16` com `qemu-system-x86_64`. |

O alvo padrão é Linux x86-64:

```inox
register counter: u64 = 3 @rbx

loop:
    sub counter, 1
    compare counter, 0
    jump_if not_zero, loop
    halt
```

Ele oferece os tipos `u8`, `u16`, `u32` e `u64`; instruções `move`, `add`, `sub`, `compare`, `jump`, `jump_if` e `halt`; e pinning opcional para controlar registradores x86-64.

## Limites atuais

- `boot16` gera uma imagem de disquete BIOS x86 em modo real de 16 bits. Não é um alvo UEFI nem um kernel em modo protegido/long mode.
- O carregador automático de `stage2` usa geometria de disquete 1,44 MiB (18 setores por trilha, 2 cabeças). Para outros dispositivos ou layouts, escreva a rotina em `asm`.
- O `stage2` automático é carregado para memória convencional abaixo de `0xa0000`; o limite prático é aproximadamente 608 KiB.
- Para transição de modo, acesso a hardware ou instruções ainda não expostas pela sintaxe Inox, use `asm` com as instruções NASM correspondentes.
- O compilador não inclui runtime nem biblioteca padrão. Segmentos, interrupções, protocolo de boot e layout de memória continuam explícitos no programa.

## Desenvolvimento

```sh
go test ./...
go vet ./...
```

O compilador não depende de bibliotecas Go externas. O pipeline está separado em lexer, parser, AST, análise semântica, IR, backends e CLI. Exemplos adicionais ficam em [`examples/`](examples/).
