# ss2vault

Utility to migrate Delinea (aka Thycotic) Secret Server static secrets to
Vault's KV v2 engine. More generally, it may be used to import any CSV data to
Vault, provided that it is appropriately formatted.

CSV files are processed separately due to the schema differences that may be
present in the source data. Individual secrets are dictated by CSV rows, while
column headers dictate the "key" in resulting key/value pairs.

The "Folder" value in the CSV source is used to determine the path within the
KV engine where the secret (with secret name dictated by "Secret Name" in the
source) is stored in Vault.

For example, consider the following CSV data structure:

```none
Secret Name,username,password,notes,Folder
my test account,admin,supersecret,details about this secret,\path\to\this\secret
```

After importing to the Vault KV v2 engine path at mountpoint `kv/`, this entry
would present as the key/value pair when retrieved via the Vault CLI:

```none
=========== Secret Path ===========
kv/data/path/to/this/secret/my_test_account

======= Metadata =======
Key                Value
---                -----
created_time       2023-10-18T22:09:52.485747301Z
custom_metadata    <nil>
deletion_time      n/a
destroyed          false
version            1

============= Data =============
Key                        Value
---                        -----
notes                      details about this secret
password                   supersecret
username                   admin
```

## Build Steps

1. [Install Go](https://go.dev/doc/install)
2. Clone this repository
3. Compile the binary for your architecture:

```shell
cd ss2vault
go mod tidy
go build
```

## Prerequisites

1. CSV(s) exported from Secret Server (one CSV per data schema)
2. Initialized and unsealed Vault server
3. Preexisting kv-v2 secrets engine
4. Vault token with appropriate access to the engine
5. Vault server connectivity from the location where this utility is executed

## Usage

```none
ss2vault -vaultToken $VAULT_TOKEN -vaultNamespace myNamespace -inputCsvPath /tmp/ss_data/ -vaultKvPath kv-v2
```

```none
Usage of ss2vault:
  -inputCsvFile string
        Path to specific CSV file to be processed
  -inputCsvPath string
        Path to directory containing one or more CSV files to be processed
  -pathSourceColumn string
        CSV column header to use to determine the KV path (default "Folder")
  -secretSourceColumn string
        CSV column header to use for the created KV secret (default "Secret Name")
  -undo
        Setting this to true attempts to delete the secrets in Vault that are referenced in the CSV input file(s)
  -vaultAddr string
        Vault Address (default "http://127.0.0.1:8200")
  -vaultKvPath string
        Vault KV v2 mount path (default "kv")
  -vaultNamespace string
        Vault Namespace (default "root")
  -vaultToken string
        Vault token
  -verbose
        Setting this to true enables detailed output
```

## Notes

- Spaces in paths and filenames are converted to underscores during the import
process
- Illegal (non-ascii and non-alphanumeric) characters are stripped from paths
(not from secrets/values)
- For generic CSV data that did not originate from Secret Server, the
`-pathSourceColumn` and `-secretSourceColumn` may be used to override the
default behavior. These options should not be necessary under typical
circumstances.
