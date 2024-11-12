package main

import (
	"fmt"
	"os/exec"
)

func main() {
	// Prepare the command
	cmd := exec.Command("ls", "-la")

	// Run the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the output
	fmt.Println(string(output))
}
