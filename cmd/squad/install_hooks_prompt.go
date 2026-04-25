package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zsiec/squad/plugin/hooks"
)

func promptHook(out io.Writer, h hooks.Hook) (bool, error) {
	return promptHookWithIO(out, os.Stdin, h)
}

func promptHookWithIO(out io.Writer, in io.Reader, h hooks.Hook) (bool, error) {
	suffix := "[y/N]"
	if h.DefaultOn {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(out, "\nHook: %s\n", h.Name)
	fmt.Fprintf(out, "  what:  %s\n", h.Description)
	fmt.Fprintf(out, "  cost:  %s\n", h.TradeOff)
	fmt.Fprintf(out, "Install? %s ", suffix)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return h.DefaultOn, nil
	}
	switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
	case "":
		return h.DefaultOn, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return h.DefaultOn, nil
	}
}
