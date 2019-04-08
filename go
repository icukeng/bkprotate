export GOPATH=$(dirname $(readlink -f $0))/.go
#echo $GOPATH
/usr/lib/go-1.9/bin/go $@
