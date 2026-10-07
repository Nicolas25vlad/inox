package boot16

import (
	"fmt"
	"strings"

	"inox/compiler/ast"
)

const FloppyImageSize = 1474560

func Generate(program *ast.Program) string {
	var out strings.Builder
	stage2 := findStage2(program)
	out.WriteString("bits 16\norg 0x7c00\n\njmp 0x0000:__inox_boot_entry\n__inox_boot_entry:\n")
	out.WriteString("    cli\n    xor ax, ax\n    mov ds, ax\n    mov es, ax\n    mov ss, ax\n    mov sp, 0x7c00\n    cld\n")
	if stage2 != nil {
		out.WriteString("    mov [__inox_boot_drive], dl\n")
	}
	out.WriteString("    sti\n")

	texts := make([]string, 0)
	for _, statement := range program.Statements {
		switch node := statement.(type) {
		case ast.AssemblyBlock:
			writeAssemblyBlock(&out, node.Source)
		case ast.BIOSPrint:
			writeBIOSPrint(&out, len(texts))
			texts = append(texts, node.Text)
		}
	}

	if stage2 != nil {
		writeStage2Loader(&out)
	} else {
		writeHaltLoop(&out, "__inox_halt")
	}
	for index, value := range texts {
		fmt.Fprintf(&out, "\n__inox_text_%d: db %s\n", index, byteList(value))
	}
	if stage2 != nil {
		writeLoaderData(&out)
	}
	out.WriteString("\ntimes 510 - ($ - $$) db 0\ndw 0xaa55\n")
	if stage2 != nil {
		out.WriteString("\n__inox_stage2_start:\n")
		writeAssemblyBlock(&out, stage2.Source)
		out.WriteString("__inox_stage2_end:\n")
	}
	fmt.Fprintf(&out, "\ntimes %d - ($ - $$) db 0\n", FloppyImageSize)
	return out.String()
}

func findStage2(program *ast.Program) *ast.Stage2Block {
	for _, statement := range program.Statements {
		if node, ok := statement.(ast.Stage2Block); ok {
			return &node
		}
	}
	return nil
}

func writeAssemblyBlock(out *strings.Builder, source string) {
	out.WriteString("\n")
	out.WriteString(source)
	if !strings.HasSuffix(source, "\n") {
		out.WriteString("\n")
	}
}

func writeBIOSPrint(out *strings.Builder, index int) {
	fmt.Fprintf(out, "\n    mov si, __inox_text_%d\n__inox_print_%d:\n", index, index)
	out.WriteString("    lodsb\n    test al, al\n")
	fmt.Fprintf(out, "    jz __inox_print_done_%d\n", index)
	out.WriteString("    mov ah, 0x0e\n    mov bx, 0x0007\n    int 0x10\n")
	fmt.Fprintf(out, "    jmp __inox_print_%d\n__inox_print_done_%d:\n", index, index)
}

func writeStage2Loader(out *strings.Builder) {
	out.WriteString(`
    mov word [__inox_remaining], (__inox_stage2_end - __inox_stage2_start + 511) / 512
    mov byte [__inox_cylinder], 0
    mov byte [__inox_head], 0
    mov byte [__inox_sector], 2
    xor ax, ax
    mov es, ax
    mov bx, 0x7e00
__inox_load_sector:
    mov ax, es
    cmp ax, 0xa000
    jae __inox_disk_error
    mov byte [__inox_retries], 3
__inox_read_attempt:
    mov ah, 0x02
    mov al, 1
    mov ch, [__inox_cylinder]
    mov cl, [__inox_sector]
    mov dh, [__inox_head]
    mov dl, [__inox_boot_drive]
    push bx
    push es
    int 0x13
    pop es
    pop bx
    jnc __inox_read_ok
    mov dl, [__inox_boot_drive]
    xor ah, ah
    push bx
    push es
    int 0x13
    pop es
    pop bx
    dec byte [__inox_retries]
    jnz __inox_read_attempt
    jmp __inox_disk_error
__inox_read_ok:
    add bx, 512
    jnc __inox_next_sector
    mov ax, es
    add ax, 0x1000
    mov es, ax
__inox_next_sector:
    inc byte [__inox_sector]
    cmp byte [__inox_sector], 19
    jb __inox_decrement_count
    mov byte [__inox_sector], 1
    inc byte [__inox_head]
    cmp byte [__inox_head], 2
    jb __inox_decrement_count
    mov byte [__inox_head], 0
    inc byte [__inox_cylinder]
__inox_decrement_count:
    dec word [__inox_remaining]
    jnz __inox_load_sector
    jmp 0x0000:0x7e00
__inox_disk_error:
    mov si, __inox_disk_error_text
__inox_disk_error_print:
    lodsb
    test al, al
    jz __inox_disk_error_halt
    mov ah, 0x0e
    mov bx, 0x0007
    int 0x10
    jmp __inox_disk_error_print
__inox_disk_error_halt:
    cli
    hlt
    jmp __inox_disk_error_halt
`)
}

func writeLoaderData(out *strings.Builder) {
	out.WriteString(`
__inox_boot_drive: db 0
__inox_retries: db 0
__inox_cylinder: db 0
__inox_head: db 0
__inox_sector: db 0
__inox_remaining: dw 0
__inox_disk_error_text: db 'Inox: falha ao ler o disco', 0
`)
}

func writeHaltLoop(out *strings.Builder, label string) {
	fmt.Fprintf(out, "\n%s:\n    cli\n    hlt\n    jmp %s\n", label, label)
}

func byteList(value string) string {
	var out strings.Builder
	for i, b := range []byte(value) {
		if i != 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(&out, "0x%02x", b)
	}
	if out.Len() != 0 {
		out.WriteString(", ")
	}
	out.WriteString("0")
	return out.String()
}
