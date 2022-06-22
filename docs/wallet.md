# CLI Wallet

The CLI wallet is in `cmd/wallet`.

You need to run it with `go run . [32-byte hex priv key]`.

## Commands

- `show`: Show wallet information.
- `transfer [toAddr] [amount] [msg]`: Transfer funds.
- `deploy [elf file path]`: Deploy a contract.
- `read/write [contract] [func] [arg1] [arg2] ...`: Run a contract. `func` can be the functional name itself, or like `func[ia]`, `func(0.5 tcoin)`, `func[ia](0.5 tcoin)` to specify the signature and call value.