package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
)

var (
	verbose bool
	undo    bool
	counter = 0
)

type clientConfig struct {
	Context   context.Context
	Addr      string
	Namespace string
	Token     string
	Client    *vault.Client
}

type secretMeta struct {
	CsvFile            string
	KvPath             string
	SecretSourceColumn string
	PathSourceColumn   string
}

type secretData struct {
	Path     string
	Contents map[string]any
}

func (auth *clientConfig) Init() {
	client, err := vault.New(
		vault.WithAddress(auth.Addr),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		panic(err)
	}
	client.SetNamespace(auth.Namespace)
	client.SetToken(auth.Token)

	vaultStatus, err := client.System.SealStatus(auth.Context)
	if err != nil {
		panic(err)
	}

	fmt.Println("Found Vault:", vaultStatus.Data.ClusterName)
	fmt.Println("Initialized:", vaultStatus.Data.Initialized)
	fmt.Println("Sealed:", vaultStatus.Data.Sealed)

	authTest, err := client.Auth.TokenLookUpSelf(auth.Context)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Token Policies: %v\n", authTest.Data["policies"])
	fmt.Println("---")

	auth.Client = client
}

func (m secretMeta) process(auth clientConfig, wg *sync.WaitGroup) {
	csv, err := os.OpenFile(m.CsvFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer csv.Close()

	csvMap, err := gocsv.CSVToMaps(csv)
	if err != nil {
		panic(err)
	}

	for _, row := range csvMap {
		var s secretData
		s.Contents = make(map[string]any)

		secretPath := stringCleaning(row[m.PathSourceColumn], true)
		secretName := stringCleaning(row[m.SecretSourceColumn], false)
		s.Path = secretPath + "/" + secretName

		for k, v := range row {
			if k != m.SecretSourceColumn && k != m.PathSourceColumn && v != "" {
				s.Contents[k] = v
			}
		}
		if undo {
			s.delete(auth, m)
		} else {
			s.create(auth, m)
		}
	}
	fmt.Printf("Finished processing %s (%d secrets)\n", m.CsvFile, len(csvMap))
	counter = counter + len(csvMap)
	wg.Done()
}

func stringCleaning(s string, path bool) string {
	var re *regexp.Regexp
	s = strings.Trim(s, " ")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "\\", "/")

	if path {
		re = regexp.MustCompile("[[:^ascii:]]")
		s = strings.TrimPrefix(s, "/")
	} else {
		re = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	}
	return re.ReplaceAllLiteralString(s, "")
}

func (s secretData) create(auth clientConfig, m secretMeta) {
	if verbose {
		fmt.Printf("creating: %s with fields %v\n", s.Path, s.Contents)
	}
	_, err := auth.Client.Secrets.KvV2Write(auth.Context, s.Path, schema.KvV2WriteRequest{
		Data: s.Contents,
	}, vault.WithMountPath(m.KvPath))

	if err != nil {
		fmt.Printf("error: unable to process %s: %s\n", m.CsvFile, s.Path)
		fmt.Printf("%v\n", err)
	}
}

func (s secretData) delete(auth clientConfig, m secretMeta) {
	if verbose {
		fmt.Printf("deleting: %s\n", s.Path)
	}
	_, err := auth.Client.Secrets.KvV2DeleteMetadataAndAllVersions(auth.Context, s.Path, vault.WithMountPath(m.KvPath))

	if err != nil {
		fmt.Printf("error: unable to delete secret: %s\n", s.Path)
		fmt.Printf("%v\n", err)
	}
}

func main() {

	startTime := time.Now().Unix()

	var (
		auth         clientConfig
		secretMeta   secretMeta
		inputCsvFile string
		inputCsvPath string
		operation    = "created"
	)

	auth.Context = context.Background()

	flag.StringVar(&auth.Addr, "vaultAddr", "http://127.0.0.1:8200", "Vault Address")
	flag.StringVar(&auth.Namespace, "vaultNamespace", "root", "Vault Namespace")
	flag.StringVar(&auth.Token, "vaultToken", "", "Vault token")
	flag.StringVar(&inputCsvFile, "inputCsvFile", "", "Path to specific CSV file to be processed")
	flag.StringVar(&inputCsvPath, "inputCsvPath", "", "Path to directory containing one or more CSV files to be processed")
	flag.StringVar(&secretMeta.KvPath, "vaultKvPath", "kv", "Vault KV v2 mount path")
	flag.StringVar(&secretMeta.SecretSourceColumn, "secretSourceColumn", "Secret Name", "CSV column header to use for the created KV secret")
	flag.StringVar(&secretMeta.PathSourceColumn, "pathSourceColumn", "Folder", "CSV column header to use to determine the KV path")
	flag.BoolVar(&verbose, "verbose", false, "Setting this to true enables detailed output")
	flag.BoolVar(&undo, "undo", false, "Setting this to true attempts to delete the secrets in Vault that are referenced in the CSV input file(s)")
	flag.Parse()

	if inputCsvPath == "" && inputCsvFile == "" || auth.Token == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if strings.HasSuffix(inputCsvPath, "/") {
		strings.TrimSuffix(inputCsvPath, "/")
	}

	if inputCsvPath != "" && inputCsvFile != "" {
		fmt.Println("Only one of inputCsvPath and inputCsvFile may be specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if undo {
		operation = "deleted"
	}

	auth.Init()

	if inputCsvPath != "" {
		var wg sync.WaitGroup
		files, _ := os.ReadDir(inputCsvPath)
		fmt.Printf("Processing %d files\n", len(files))
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
				wg.Add(1)
				secretMeta.CsvFile = inputCsvPath + "/" + file.Name()
				go secretMeta.process(auth, &wg)
			}
		}
		wg.Wait()
	} else {
		var wg sync.WaitGroup
		wg.Add(1)
		secretMeta.CsvFile = inputCsvFile
		secretMeta.process(auth, &wg)
		wg.Wait()
	}
	duration := time.Now().Unix() - startTime
	fmt.Printf("Successfully %s %d secrets in %d seconds\n", operation, counter, duration)
}
