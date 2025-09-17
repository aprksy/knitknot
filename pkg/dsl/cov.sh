PKG=github.com/aprksy/knitknot/pkg/dsl
DIR=/home/aprksy/workspace/repo/git/project-db/project-knitknot/knitknot/pkg/dsl/
ginkgo -r -v -race --trace --coverpkg=$PKG --coverprofile=.coverage.tmp.out $DIR/...
go test -fuzz=FuzzParser -fuzztime=30s $DIR/
cat $DIR/.coverage.tmp.out | grep -v "parser_fuzz" | grep -v "ast" > $DIR/coverage.out
rm $DIR/.coverage.tmp.out
go tool cover -func=$DIR/coverage.out
