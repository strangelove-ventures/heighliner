build:
	cat chains/*.yaml > chains.yaml
	go build
	rm -f chains.yaml
install:
	cat chains/*.yaml > chains.yaml
	go install
	rm -f chains.yaml