package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"schemeapp/backend/internal/security"
)

func printPasswordHash(input io.Reader, output io.Writer) error {
	password, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("read password: %w", err)
	}
	password = strings.TrimSuffix(password, "\n")
	password = strings.TrimSuffix(password, "\r")

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, hash); err != nil {
		return fmt.Errorf("write password hash: %w", err)
	}
	return nil
}
