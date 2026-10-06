# Inox

Inox v0.1 is a small compiler prototype for explicit x86-64 register operations. It accepts `.ix` source and emits NASM syntax for a Linux x86-64 program.

## Build the compiler

```sh
go build -o inoxc ./cmd/inoxc
```

Compile a source file to `build/<name>.asm`:

```sh
./inoxc build examples/basic.ix
```

Inspect the intermediate representation or generated assembly:

```sh
./inoxc inspect examples/basic.ix --ir
./inoxc inspect examples/basic.ix --asm
```

Assemble, link, and execute a program natively or through QEMU user-mode:

```sh
./inoxc run examples/basic.ix
./inoxc run examples/basic.ix --vm
```

`run --vm` looks for `qemu-x86_64` or `qemu-x86_64-static` in `PATH`. Both run the generated Linux ELF as a guest process; they do not boot a virtual machine.

Assembling and linking require `nasm` and GNU `ld` in `PATH`.

The v0.1 language includes `u8`, `u16`, `u32`, and `u64` virtual registers; `move`, `add`, `sub`, `compare`, `jump`, `jump_if`, and `halt`. Up to four virtual registers are allocated to `rax`, `rbx`, `rcx`, and `rdx` (using the matching width aliases). Register pinning syntax is parsed but reports that pinning is not implemented yet.

`halt` emits the Linux x86-64 exit syscall. The generated NASM file can be assembled and linked with:

```sh
nasm -felf64 build/basic.asm -o build/basic.o
ld -o build/basic build/basic.o
./build/basic
```

Run compiler tests with:

```sh
go test ./...
```

The next milestone is bootable output for system emulation in QEMU. The current output is a Linux user-space program, not a boot image.
