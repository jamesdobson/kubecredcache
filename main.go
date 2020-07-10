package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	initialize()

	if len(os.Args) <= 1 {
		log.Fatal("kubecredcache requires at least one argument.")
	}

	var commandName = os.Args[1]
	var commandArgs = os.Args[2:]

	log.Printf("🐢  Cache is empty; calling '%s'...", commandName)
	run(commandName, commandArgs)
}

func initialize() {
	log.SetFlags(0)
}

// Execute the command with the given arguments. Redirect standard streams
// from this process to the command, use the same environment, and exit this
// process with whichever code the command exits.
func run(name string, args []string) {
	var cmd = exec.Command(name, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		code := cmd.ProcessState.ExitCode()
		log.Printf("\n'%s' exited with code: %d\n", name, code)
		os.Exit(code)
	}
}
