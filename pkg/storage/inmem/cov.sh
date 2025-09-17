PKG=github.com/aprksy/knitknot/pkg/storage/inmem
DIR=/home/aprksy/workspace/repo/git/project-db/project-knitknot/knitknot/pkg/storage/inmem
ginkgo -r -v -race --trace --coverpkg=$PKG --coverprofile=.coverage.tmp.out $DIR/...
cat $DIR/.coverage.tmp.out | grep -v "resultset" > $DIR/coverage.out
rm $DIR/.coverage.tmp.out
go tool cover -func=$DIR/coverage.out
