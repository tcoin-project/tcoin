# TCoin VM

TCoin VM is based on RISC-V.

## Instructions
All `rv64im` instructions are implemented.

## Memory
In a type 2 transactions, there could be at most 256 contracts loaded in the memory.

Each contract has its own memory segment `0xXXXXXXXX00000000`, and currently the `XXXXXXXX` is the number between 0 and 255.

Each contract can map different types of memory. Let the memory address be `0xXXXXXXXXYZZZZZZZ`, and `Y` indicates the memory type. `Y=1` means self executable, everyone can read. `Y=2` and `Y=3` means only the contract itself can read/write. `Y=4` and `Y=5` means, the contract itself can write, and all others can read.

When a contract reads some unallocated memory, it will be allocated automatically, and the gas fee is deduced.

The contracts are compiled into PIE ELF, and loaded with base address `0xXXXXXXXX00000000`, so we should set the text segment to `0x10000000` with some offset for the ELF header.

## Calling Other Contracts

At the entry point of a deployed contract, it can execute some initialization code, and returns a new entry point. When other contracts loads this contract, the new entry point is cached and returned, and callings will be done through this.

The default calling convention is `uint64_t (uint64_t selector, uint64_t args)`, and when there are `>=1` arguments, or the arguments can't fit into a `uint64_t`, they will be put into the shared memory, the then a pointer is passed to the callee.

In order to prevent ROP attacks, each calling destination must be registered.

Besides normal calling, there is a protected call, and one can set value, gas limit of the call, and handle errors as well.

Here is an example of the loading process:

A loads B -> execute B init -> A get entry of B -> A can call B.

C loads B -> C get the cached entry of B -> C can call B.

## Storage Model

Like Ethereum, each account has a 32-byte to 32-byte mapping.

## Syscall

Some syscalls are implemented to access the chain, and reduce gas usage for some operations (like hashing). The syscall convention is not the standard one, but like the `vdso` and `vsyscall` in Linux. There are some pseudo functions around `0x7fff...`, and when pc goes there, the VM interpreter will execute the corresponding syscall.

Syscall list:

| ID   | Name           | C definition                                                 | Explanation                                                  |
| ---- | -------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 1    | SELF           | `void (Address *self)`                                       | Get the address of self.                                     |
| 2    | ORIGIN         | `void (Address *origin)`                                     | Get the address of the transaction origin.                   |
| 3    | CALLER         | `void (Address *caller)`                                     | Get the address of the caller.                               |
| 4    | CALLVALUE      | `uint64_t ()`                                                | Get the value of this call.                                  |
| 5    | STORAGE_STORE  | `void (const char *key, const char *value)`                  | Store something.                                             |
| 6    | STORAGE_LOAD   | `void (const char *key, char *value)`                        | Load something.                                              |
| 7    | SHA256         | `void (const char *key, size_t len, char *res)`              | Calculate SHA-256.                                           |
| 8    | BALANCE        | `uint64_t (const Address *addr)`                             | Get the balance of some account.                             |
| 9    | LOAD_CONTRACT  | `void* (const Address *addr)`                                | Load a contract into memory, return its entry point.         |
| 10   | PROTECTED_CALL | `uint64_t (void *(call)(uint64_t, void *), uint64_t a1, void *a2, uint64_t value, uint64_t gasLimit, bool *success, char *errorMsg)` | Execute `call(a1, a2)` with the value and gas limit, return errors if any happened. |
| 11   | REVERT         | `void (const char *msg)`                                     | Revert with message (capped to 1024 bytes).                  |
| 12   | TIME           | `uint64_t ()`                                                | Get current time.                                            |
| 13   | MINER          | `void (Address *miner)`                                      | Get the miner of the block.                                  |
| 14   | BLOCK_NUMBER   | `uint64_t ()`                                                | Get the block number.                                        |
| 15   | DIFFICULTY     | `void (Address *difficulty)`                                 | Get the difficulty of the block.                             |
| 16   | CHAINID        | `uint64_t ()`                                                | Get the chain id.                                            |
| 17   | GAS            | `uint64_t ()`                                                | Get remaining gas.                                           |
| 18   | JUMPDEST       | `void (void *addr)`                                          | Mark an address as a valid inter-contract calling destination. This syscall can only be used in the init stage. |
| 19   | TRANSFER       | `void (const Address *addr, uint64_t value, const char *msg, size_t msgLen)` | Transfer funds. This acts the same as an type 1 tx.          |
| 20   | CREATE         | `void (Address *res, const char *code, size_t len, uint64_t flags, uint64_t nonce)` | Create a contract.                                           |
| 21   | ED25519_VERIFY | `bool (const char *msg, size_t len, const char *pubkey, const char *sig)` | Verify a Ed25519 signature.                                  |
| 22   | LOAD_ELF       | `void* (const Address *addr, size_t *offset)`                | Load another ELF into current address space. This can be used to make proxies. |

