gocovmerge $(find . -name "*.out") > merged.coverprofile
go tool cover -func=merged.coverprofile
go tool cover -html=merged.coverprofile -o coverage.html