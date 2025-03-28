build:
	cat chains/*.yaml > chains.yaml
	go build
	echo "# This is a stub file. Please add new chains to the chains/ directory" > chains.yaml
install:
	cat chains/*.yaml > chains.yaml
	go install
	echo "# This is a stub file. Please add new chains to the chains/ directory" > chains.yaml