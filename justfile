lint:
    golangci-lint run

generate-cheatsheets:
    go run scripts/cheatsheet/main.go generate
