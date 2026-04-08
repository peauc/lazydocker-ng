lint:
    golangci-lint run

start-projects:
    docker compose -f test/project-a.yml up -d
    docker compose -f test/project-b.yml up -d
    docker compose -f test/project-c.yml up -d


generate-cheatsheets:
    go run scripts/cheatsheet/main.go generate
