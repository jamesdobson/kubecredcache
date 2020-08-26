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

	"gopkg.in/yaml.v2"
)

const (
	ProgramName                 = "kubecredcache"
	ExpireEarlyBySeconds        = 120
	AccessKeyEnvVarName         = "AWS_ACCESS_KEY_ID"
	CacheFileGCThresholdSeconds = 20 * 60
)

var (
	configDir string
)

// TODO: Implement build and test/lint
// TODO: Publish to Homebrew

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
	gc()

	if len(os.Args) <= 1 {
		log.Fatalf("%s requires at least one argument.", ProgramName)
	}

	commandName := os.Args[1]
	commandArgs := os.Args[2:]

	switch commandName {
	case "--install":
		install(commandArgs)
	default:
		mainAction(commandName, commandArgs)
	}

	os.Exit(0)
}

func mainAction(commandName string, commandArgs []string) {
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
	log.Printf("⚡️  token is now cached!\n")
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
}

func install(args []string) {
	if len(args) != 1 {
		log.Fatalf("The --install command takes one and only one argument: the path to the kubeconfig file\n")
	}

	configFileName := args[0]
	data, err := ioutil.ReadFile(configFileName)
	if err != nil {
		log.Fatalf("Unable to open file '%s': %v\n", configFileName, err)
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		log.Fatalf("Unable to process file '%s': %v\n", configFileName, err)
	}

	// modify to use this program
	users := m["users"].([]interface{})
	userEntry := users[0].(map[interface{}]interface{})
	user := userEntry["user"].(map[interface{}]interface{})
	exec := user["exec"].(map[interface{}]interface{})
	command := exec["command"].(string)

	if command == ProgramName {
		log.Fatalf("%s is already installed in '%s'\n", ProgramName, configFileName)
	}

	exec["command"] = ProgramName
	commandArgs := exec["args"].([]interface{})
	commandArgs = append([]interface{}{command}, commandArgs...)
	exec["args"] = commandArgs

	// write back to the file
	data, err = yaml.Marshal(&m)
	if err != nil {
		log.Fatalf("Unable to marshal: %v\n", err)
	}

	err = ioutil.WriteFile(configFileName, data, 0600)
	if err != nil {
		log.Fatalf("Unable to write file '%s': %v\n", configFileName, err)
	}

	log.Printf("✅  %s installed in '%s'.\n", ProgramName, configFileName)
}

func gc() {
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		log.Printf("%s gc failed: %v\n", ProgramName, err)
		return
	}

	gcThreshold := time.Now().Unix() - CacheFileGCThresholdSeconds

	for _, f := range files {
		if f.ModTime().Unix() < gcThreshold {
			err := os.Remove(path.Join(configDir, f.Name()))
			if err != nil {
				log.Printf("%s gc failed to remove '%s': %v\n", ProgramName, f.Name(), err)
			}
		}
	}
}
