PKG=github.com/aprksy/knitknot/pkg/graph
DIR=/home/aprksy/workspace/repo/git/project-db/project-knitknot/knitknot/pkg/graph
ginkgo -r -v -race --trace --coverpkg=$PKG --coverprofile=.coverage.tmp.out $DIR/...
cat $DIR/.coverage.tmp.out > $DIR/coverage.out
rm $DIR/.coverage.tmp.out
go tool cover -func=$DIR/coverage.out
