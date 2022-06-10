#!/bin/sh
rm $1
riscv64-elf-gcc \
    tcoin.cpp stdlib.cpp \
    $1.cpp -o $1 \
    -nostdlib -nodefaultlibs -fno-builtin \
    -std=c++20 \
    -march=rv64im -mabi=lp64 \
    -Os -Wl,--gc-sections -flto \
    -fPIE \
    -Ttext 0x10000190 \
    -Wl,--section-start,.private_data=0x20000000 \
    -Wl,--section-start,.shared_data=0x40000000 \
    -Wl,--section-start,.init_code=0x100FF000 \
    -s
#riscv64-elf-gcc tcoin.cpp tcoin.S stdlib.cpp test.cpp -o test -nostdlib -fno-builtin -Os -march=rv64im -mabi=lp64
#clang --target=riscv64 tcoin.c tcoin.S test.c -o test -nostdlib -fno-builtin -O3 -march=rv64im -mabi=lp64 -fuse-ld=mold -Wl,--gc-sections -Ttext 0x20000
riscv64-elf-objcopy --remove-section .eh_frame $1
riscv64-elf-objdump -d $1