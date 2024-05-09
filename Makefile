LDFLAGS="-X main.Buildstamp=`date '+%Y-%m-%d_%I:%M:%S%p'` -X main.Githash=`git describe --tags` -s -w"

build: clean
	go mod tidy
	go build -ldflags $(LDFLAGS) -o ./cfddns main.go
 

# install:
# 	mkdir -vp /usr/local/bin/
# 	cp output/gobeat /usr/local/bin/


clean:
	rm -rf ./cfddns