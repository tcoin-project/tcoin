# Account

Each account have a 32-byte long address, and it's the SHA-256 of something.

For an EOA (Externally owned account), it's the SHA-256 of an Ed25519 public key.

For a contract, it's the SHA-256 of contract code and something else. You can refer [this](../core/block/vm_syscall.go) for more information.

EOAs can transfer funds to other accounts (type 1 tx), and they can also execute code on their own (type 2 tx). In type 2 txs, they can emit contracts, and the contracts can also do that. All accounts can also emit type 1 tx in code.