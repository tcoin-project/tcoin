# Fullnode Setup

You need to download the [global_config.json](global_config.json), and create a `config.json` (you may refer to [sample_config.json](sample_config.json)).

Then, go to `/cmd/fullnode`, and run the following command to start syncing.

```shell
go run main.go -config /path/to/config.json -globalConfig /path/to/global_config.json
```

## Config Explanation
### Global Config
The global config contains the chain id (like Ethereum), a genesis block, a genesis consensus state (which contains difficulty), and a bootstrap peer address.

### Config
- `storage_path`: The path of all generated files, including the database and peer information.
- `storage_finalize_depth`: Max depth supported for reorgs. Currently some functions have linear complexity depending on this, so 30 is a resonable choice.
- `storage_dump_disk_ratio`: Expected time usage for dumping the memory database to the disk.
- `listen_port`: The port to listen to other peers. You can use `-1` for a local testing chain.
- `max_connections`: Maximum number of connections.