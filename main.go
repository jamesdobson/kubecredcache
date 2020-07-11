package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/spf13/viper"
)

const (
	ProgramName = "kubecredcache"
)

var (
	configDir string
)

func main() {
	initialize()

	if len(os.Args) <= 1 {
		log.Fatalf("%s requires at least one argument.", ProgramName)
	}

	var commandName = os.Args[1]
	var commandArgs = os.Args[2:]

	data := getCacheData()

	if data != "" {
		log.Println("⚡️  Cache hit; enjoy!")
		_, err := os.Stdout.WriteString(data)
		if err != nil {
			log.Fatalf("Error returning cached data: %v\n", err)
		}

		os.Exit(0)
	}

	log.Printf("🐢  Cache miss; calling '%s'...\n", commandName)

	output := run(commandName, commandArgs)

	putCacheData(output)
}

func getCacheData() string {
	data, err := ioutil.ReadFile(path.Join(configDir, "cache.bin"))

	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}

		log.Fatalf("Error reading cache: %v\n", err)
	}

	return string(data)
}

func putCacheData(output string) {
	f, err := os.Create(path.Join(configDir, "cache.bin"))

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
