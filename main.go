package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
)

func initVault(ctx context.Context, vaultAddr string, vaultNamespace string, vaultToken string) *vault.Client {
	client, err := vault.New(
		vault.WithAddress(vaultAddr),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		panic(err)
	}
	client.SetNamespace(vaultNamespace)
	client.SetToken(vaultToken)

	vaultStatus, err := client.System.SealStatus(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Found Vault:", vaultStatus.Data.ClusterName)
	fmt.Println("Initialized:", vaultStatus.Data.Initialized)
	fmt.Println("Sealed:", vaultStatus.Data.Sealed)

	authTest, err := client.Auth.TokenLookUpSelf(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Token Policies: %v\n", authTest.Data["policies"])
	fmt.Println("---")

	return client
}

func processCsv(ctx context.Context, inputCsvFile string, vaultKvPath string, client *vault.Client, verbose bool, undo bool) int {
	csv, err := os.OpenFile(inputCsvFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer csv.Close()

	csvMap, err := gocsv.CSVToMaps(csv)
	if err != nil {
		panic(err)
	}

	for _, row := range csvMap {
		secretContent := make(map[string]string)
		reName := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
		secretName := strings.ReplaceAll(strings.Trim(row["Secret Name"], " "), " ", "_")
		secretName = reName.ReplaceAllLiteralString(secretName, "")
		rePath := regexp.MustCompile("[[:^ascii:]]")
		secretPath := strings.TrimPrefix(strings.ReplaceAll(strings.Trim(strings.ReplaceAll(row["Folder"], "\\", "/"), " "), " ", "_"), "/")
		secretPath = rePath.ReplaceAllLiteralString(secretPath, "")
		for k, v := range row {
			if k != "Secret Name" && k != "Folder" && v != "" {
				secretContent[k] = v
			}
		}
		if undo {
			if verbose {
				fmt.Printf("deleting: %s\n", secretPath+"/"+secretName)
			}
			deleteKvSecret(ctx, client, inputCsvFile, vaultKvPath, secretPath+"/"+secretName)
		} else {
			if verbose {
				fmt.Printf("creating: %s with fields %v\n", secretPath+"/"+secretName, secretContent)
			}
			createKvSecret(ctx, client, inputCsvFile, vaultKvPath, secretPath+"/"+secretName, secretContent)
		}
	}
	return len(csvMap)
}

func createKvSecret(ctx context.Context, client *vault.Client, inputCsvFile string, vaultKvPath string, secretPath string, secretContent map[string]string) {
	secretInt := make(map[string]interface{}, len(secretContent))
	for k, v := range secretContent {
		secretInt[k] = v
	}
	_, err := client.Secrets.KvV2Write(ctx, secretPath, schema.KvV2WriteRequest{
		Data: secretInt,
	}, vault.WithMountPath(vaultKvPath))

	if err != nil {
		fmt.Printf("error: unable to process %s: %s\n", inputCsvFile, secretPath)
		fmt.Printf("%v\n", err)
	}
}

func deleteKvSecret(ctx context.Context, client *vault.Client, inputCsvFile string, vaultKvPath string, secretPath string) {
	_, err := client.Secrets.KvV2DeleteMetadataAndAllVersions(ctx, secretPath, vault.WithMountPath(vaultKvPath))

	if err != nil {
		fmt.Printf("error: unable to delete secret: %s\n", secretPath)
		fmt.Printf("%v\n", err)
	}
}

func main() {

	var (
		vaultAddr      string
		vaultNamespace string
		vaultToken     string
		inputCsvFile   string
		inputCsvPath   string
		vaultKvPath    string
		verbose        bool
		undo           bool
	)

	flag.StringVar(&vaultAddr, "vaultAddr", "http://127.0.0.1:8200", "Vault Address")
	flag.StringVar(&vaultNamespace, "vaultNamespace", "root", "Vault Namespace")
	flag.StringVar(&vaultToken, "vaultToken", "", "Vault token")
	flag.StringVar(&inputCsvFile, "inputCsvFile", "", "Path to specific CSV file to be processed")
	flag.StringVar(&inputCsvPath, "inputCsvPath", "", "Path to directory containing one or more CSV files to be processed")
	flag.StringVar(&vaultKvPath, "vaultKvPath", "kv", "Vault KV v2 mount path")
	flag.BoolVar(&verbose, "verbose", false, "Setting this to true enables detailed output")
	flag.BoolVar(&undo, "undo", false, "Setting this to true attempts to delete the secrets in Vault that are referenced in the CSV input file(s)")
	flag.Parse()

	if inputCsvPath == "" && inputCsvFile == "" || vaultToken == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if inputCsvPath != "" && inputCsvFile != "" {
		fmt.Println("Only one of inputCsvPath and inputCsvFile may be specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx := context.Background()
	client := initVault(ctx, vaultAddr, vaultNamespace, vaultToken)

	if inputCsvPath != "" {
		files, _ := os.ReadDir(inputCsvPath)
		fmt.Printf("Found %d files\n", len(files))
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
				fmt.Printf("Parsing CSV %s... ", file.Name())
				numSecrets := processCsv(ctx, inputCsvPath+"/"+file.Name(), vaultKvPath, client, verbose, undo)
				fmt.Printf("Complete (processed %d secrets)\n", numSecrets)
			}
		}
	} else {
		processCsv(ctx, inputCsvFile, vaultKvPath, client, verbose, undo)
	}
}
