#ifndef _SYSCALL_H
#define _SYSCALL_H

#include "stdlib.h"
const int SYSCALL_SELF = 1;
const int SYSCALL_ORIGIN = 2;
const int SYSCALL_CALLER = 3;
const int SYSCALL_CALLVALUE = 4;
const int SYSCALL_STORAGE_STORE = 5;
const int SYSCALL_STORAGE_LOAD = 6;
const int SYSCALL_SHA256 = 7;
const int SYSCALL_BALANCE = 8;
const int SYSCALL_LOAD_CONTRACT = 9;
const int SYSCALL_PROTECTED_CALL = 10;
const int SYSCALL_REVERT = 11;
const int SYSCALL_TIME = 12;
const int SYSCALL_MINER = 13;
const int SYSCALL_BLOCK_NUMBER = 14;
const int SYSCALL_DIFFICULTY = 15;
const int SYSCALL_CHAINID = 16;
const int SYSCALL_GAS = 17;
const int SYSCALL_JUMPDEST = 18;
const int SYSCALL_TRANSFER = 19;
const int SYSCALL_CREATE = 20;
const int SYSCALL_ED25519_VERIFY = 21;
const int SYSCALL_LOAD_ELF = 22;

const uint64_t CREATE_TRIMELF = 1;
const uint64_t CREATE_INIT = 2;
const uint64_t CREATE_USENONCE = 4;

#endif