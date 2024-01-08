package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
)

var (
	varPattern        = regexp.MustCompile(`^\w+=\w+$`)
	varCommandPattern = regexp.MustCompile(`^(\w+)=(\$\((.+)\))$`)
)

func setCommandToVar(cmd string) error {
	if matches := varCommandPattern.FindStringSubmatch(cmd); len(matches) == 4 {
		variable, command := matches[1], matches[3]

		out, err := exec.Command("sh", "-c", command).Output()
		if err != nil {
			return fmt.Errorf("error executing command '%v': %v", command, err.Error())
		}

		err = os.Setenv(variable, strings.TrimSpace(string(out)))
		if err != nil {
			return fmt.Errorf("error setting env var '%v': %v", cmd, err.Error())
		}
	}

	return nil
}

func setVar(cmd string) error {
	parts := strings.SplitN(cmd, "=", 2)

	err := os.Setenv(parts[0], parts[1])
	if err != nil {
		return fmt.Errorf("error setting env var '%v': %v", cmd, err.Error())
	}

	return nil
}

func runCommand(cmd string) error {
	err := exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		return fmt.Errorf("error executing command '%v': %v", cmd, err.Error())
	}

	return nil
}

func runCommands(preCommands []string) error {
	for _, cmd := range preCommands {
		log.Println("running pre-command:", cmd)
		if varCommandPattern.MatchString(cmd) {
			if err := setCommandToVar(cmd); err != nil {
				return err
			}
		} else if varPattern.MatchString(cmd) {
			if err := setVar(cmd); err != nil {
				return err
			}
		} else {
			if err := runCommand(cmd); err != nil {
				return err
			}
		}
	}

	return nil
}

func Start(commandsFile string) error {
	m, err := NewDemoManager(commandsFile)
	if err != nil {
		return err
	}

	err = runCommands(m.preCommands)
	if err != nil {
		return err
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	r := mux.NewRouter()
	r.HandleFunc(indexEndpoint, m.indexHandler)
	r.HandleFunc(initEndpoint, m.initHandler)
	r.HandleFunc(pageEndpoint, m.incPageHandler).
		Methods(http.MethodPost)
	r.HandleFunc(pageEndpoint, m.decPageHandler).
		Methods(http.MethodDelete) // At least `PUT /page` is idempotent :)
	r.HandleFunc(pageEndpoint, m.setPageHandler).
		Methods(http.MethodPut)
	r.HandleFunc(executeEndpoint, m.executeCommandHandler).
		Methods(http.MethodPost)
	r.HandleFunc(executeEndpoint, m.executeStatusHandler).
		Methods(http.MethodGet)
	r.HandleFunc(executeEndpoint, m.stopCommandHandler).
		Methods(http.MethodDelete)

	http.Handle(indexEndpoint, r)
	log.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)

	return nil
}
