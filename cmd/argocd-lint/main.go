package main

import (
    "os"

    "github.com/argocd-lint/argocd-lint/internal/cli"
)

func main() {
    code := cli.Execute(os.Args[1:], os.Stdout, os.Stderr)
    os.Exit(code)
}
