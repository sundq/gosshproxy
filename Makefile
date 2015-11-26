OUTPUT_DIR = ./builds

dbsshproxy: main.go readers.go sshproxy.go 
	go build -o dbsshproxy

tools:
	go get github.com/tools/godep
	go get github.com/mitchellh/gox
	go get github.com/tcnksm/ghr

cross_compile:
	GOARM=5 gox -os="darwin linux" -arch="386 amd64" -output "${OUTPUT_DIR}/pkg/{{.OS}}_{{.Arch}}/{{.Dir}}"

shasums:
	cd ${OUTPUT_DIR}/dist; shasum * > ./SHASUMS
