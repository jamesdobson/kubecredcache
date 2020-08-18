package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/spf13/viper"
)

const (
	ProgramName          = "kubecredcache"
	ExpireEarlyBySeconds = 120
	AccessKeyEnvVarName  = "AWS_ACCESS_KEY_ID"
)

var (
	configDir string
)

// TODO: Implement garbage collection of cache directory

type CacheKey struct {
	ClusterID string
	Region    string
	AccessKey string
}

type AWSCacheEntry struct {
	Kind       string
	APIVersion string
	Status     AWSCacheEntryStatus
}

type AWSCacheEntryStatus struct {
	ExpirationTimestamp string
	Token               string
}

func main() {
	initialize()

	if len(os.Args) <= 1 {
		log.Fatalf("%s requires at least one argument.", ProgramName)
	}

	commandName := os.Args[1]
	commandArgs := os.Args[2:]
	accessKeyID := os.Getenv(AccessKeyEnvVarName)

	if accessKeyID == "" {
		log.Fatalf("%s environment variable is not set\n", AccessKeyEnvVarName)
	}

	key := getCacheKey(commandName, commandArgs, accessKeyID)
	data := getCacheData(key)
	var expired bool = false

	if data != "" {
		expired = isExpired(data)

		if !expired {
			log.Printf("⚡️  %s cache hit; enjoy!\n", ProgramName)

			_, err := os.Stdout.WriteString(data)

			if err != nil {
				log.Fatalf("Error returning cached data: %v\n", err)
			}

			os.Exit(0)
		}
	}

	var missReason string

	if expired {
		missReason = "token expired"
	} else {
		missReason = "cache empty"
	}

	log.Printf("🐢  %s cache miss (%s); calling '%s'...\n", ProgramName, missReason, commandName)

	output := run(commandName, commandArgs)

	putCacheData(output, key)
	os.Exit(0)
}

func getCacheFileName(key CacheKey) string {
	if key.ClusterID == "" || key.AccessKey == "" {
		log.Fatalf("CacheKey (%v) is missing required fields\n", key)
	}

	if key.Region == "" {
		return fmt.Sprintf("%s_%s", key.ClusterID, key.AccessKey)
	}

	return fmt.Sprintf("%s_%s_%s", key.ClusterID, key.AccessKey, key.Region)
}

func getCacheKey(command string, args []string, id string) CacheKey {
	if id == "" {
		log.Panicln("Unable to determine user access key id")
	}

	key := parseCacheKey(command, args)
	key.AccessKey = id

	if key.ClusterID == "" {
		log.Fatalf("Unable to determine cluster id from command: %s %v\n", command, args)
	}

	return key
}

func parseCacheKey(command string, args []string) CacheKey {
	var key = CacheKey{}

	if command == "aws" {
		for i := 0; i < len(args); i++ {
			arg := args[i]

			if arg == "--region" {
				i++

				if i < len(args) {
					key.Region = args[i]
				}
			} else if arg == "--cluster-name" {
				i++

				if i < len(args) {
					key.ClusterID = args[i]
				}
			}
		}
	} else if command == "aws-iam-authenticator" {
		for i := 0; i < len(args); i++ {
			arg := args[i]

			if arg == "-i" || arg == "--cluster-id" {
				i++

				if i < len(args) {
					key.ClusterID = args[i]
				}
			}
		}
	}

	return key
}

func isExpired(data string) bool {
	ts := parseExpiry(data)

	return ts.Unix()-time.Now().Unix() < ExpireEarlyBySeconds
}

func parseExpiry(data string) *time.Time {
	var cacheEntry AWSCacheEntry
	err := json.Unmarshal([]byte(data), &cacheEntry)

	if err != nil {
		log.Printf("Unable to parse cache entry: %v\n", err)
		return nil
	}

	timestamp := cacheEntry.Status.ExpirationTimestamp

	ts, err := time.Parse(time.RFC3339, timestamp)

	if err != nil {
		log.Printf("Unable to parse expiration timestamp ('%s'): %v\n", timestamp, err)
		return nil
	}

	return &ts
}

func getCacheData(key CacheKey) string {
	fileName := getCacheFileName(key)
	data, err := ioutil.ReadFile(path.Join(configDir, fileName))

	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}

		log.Fatalf("Error reading cache: %v\n", err)
	}

	return string(data)
}

func putCacheData(output string, key CacheKey) {
	fileName := getCacheFileName(key)
	f, err := os.Create(path.Join(configDir, fileName))

	if err != nil {
		log.Fatalf("Error opening cache for writing: %v\n", err)
	}

	defer f.Close()

	_, err = f.WriteString(output)

	if err != nil {
		log.Fatalf("Error writing cache: %v\n", err)
	}
}

// Execute the command with the given arguments. Redirect standard streams
// from this process to the command, use the same environment, and exit this
// process with whichever code the command exits.
func run(name string, args []string) string {
	var buf bytes.Buffer
	var out = io.MultiWriter(os.Stdout, &buf)
	var cmd = exec.Command(name, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		code := cmd.ProcessState.ExitCode()
		log.Printf("\n'%s' exited with code: %d\n", name, code)
		os.Exit(code)
	}

	return buf.String()
}

func initialize() {
	log.SetFlags(0)

	// Load Configuration
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}
	configDir = path.Join(home, "."+ProgramName)

	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(configDir, 0755)
			if err != nil {
				log.Fatalf("Unable to create configuration directory '%s': %v\n", configDir, err)
			}
		} else {
			log.Fatalf("Error getting configuration directory '%s': %v", configDir, err)
		}
	}

	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetEnvPrefix(ProgramName)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			// missing config file is ok
		default:
			log.Fatalf("Error reading '%s': %v\n", viper.ConfigFileUsed(), err)
		}
	}
}
