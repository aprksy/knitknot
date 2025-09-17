# Contributing to KnitKnot

Thank you for considering contributing! We welcome bug reports, fixes, and docs improvements.

## Development Setup
```bash
git clone https://github.com/aprksy/knitknot
cd knitknot
go mod tidy
```

## Running Tests
```bash
# Run all tests
go test ./...

# With coverage
go test -coverprofile=c.out ./...
go tool cover -html=c.out
```

## General rule:
- Let's be nice like you're always be

## Code Style 

- Follow Go idioms (use `gofmt`, `golangci-lint`)
- Write clear, documented code
- Add tests for new features
     

## Submitting Changes 

1. Fork the repo
2. Create a branch: feat/dot-export
3. Commit your changes
4. Push and open a PR
5. Describe what changed and why
     

## Issues 

Please include: 
1. Version info (knitknot version)
2. What are you trying to do
3. Isolate the problem if you can
4. Expected vs actual behavior
5. Steps to reproduce

## Feature requests

Please include:
1. The benefit to have it
2. The risk if we don't have it
3. Effort estimation
4. Whether you like to contribute for the efforts
     

We aim to respond within 7 days. 