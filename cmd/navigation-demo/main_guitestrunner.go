//go:build guitestrunner

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/VinceLewis/gio-kit/guitest/scriptcli"
)

func main() {
	result, runErr := scriptcli.Run(context.Background(), os.Args[1:], demoScriptConfig(openTemporaryDemo))
	encodeErr := json.NewEncoder(os.Stdout).Encode(result)
	if err := errors.Join(runErr, encodeErr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
