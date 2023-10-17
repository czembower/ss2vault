# ss2vault

Utility to migrate Delinea (aka Thycotic) Secret Server static secrets to Vault's KV v2 engine.

## Prerequisites

1. CSV(s) exported from Secret Server
2. Initialized and unsealed Vault server
3. Preexisting kv-v2 secrets engine
4. Vault token with appropriate access to the engine

## Usage

```shell
ss2vault -vaultToken $VAULT_TOKEN -vaultNamespace myNamespace -inputCsvPath /tmp/ss_data/ -vaultKvPath kv-v2
```

```shell
  -inputCsvFile string
        Path to specific CSV file to be processed
  -inputCsvPath string
        Path to directory containing one or more CSV files to be processed
  -vaultAddr string
        Vault Address (default "http://127.0.0.1:8200")
  -vaultKvPath string
        Vault KV v2 mount path (default "kv")
  -vaultNamespace string
        Vault Namespace (default "root")
  -vaultToken string
        Vault token
```