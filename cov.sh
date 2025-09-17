go test -coverpkg=github.com/aprksy/knitknot/pkg/... -coverprofile=coverage.out ./... 

go tool cover -func=coverage.out | \
    grep -v "^cmd/" | \
    grep -v "^internal/util" | \
    grep -v "^exporter" | \
    grep -v "^ast.go" | \
    grep -v "^parser_fuzz.go" | \
    tail -n 1 > coverage.out

