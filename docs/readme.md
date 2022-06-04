# TCoin Docs

## Fullnode Setup

You need to download the [global_config.json](global_config.json), and create a `config.json` (you may refer to [sample_config.json](sample_config.json)).

Then, go to `/cmd/fullnode`, and run the following command to start syncing.

```shell
go run main.go -config /path/to/config.json -globalConfig /path/to/global_config.json
```