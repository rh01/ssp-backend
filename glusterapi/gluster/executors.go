package gluster

import (
	"errors"
	"log"
	"strings"
)

func executeCommandsLocally(commands []string) error {
	log.Println("Got new commands to execute:")
	for _, c := range commands {
		out, err := ExecRunner.Run("bash", "-c", c)
		if err != nil {
			// If lvextend has the same size exit code 5 is fine
			if !(strings.Contains(c, "lvextend") && strings.Contains(err.Error(), "exit status 5")) {
				log.Println("Error executing command: ", c, err.Error(), string(out))
				return errors.New(commandExecutionError)
			}
		}
		log.Printf("Cmd: %v | StdOut: %v", c, string(out))
	}

	return nil
}
