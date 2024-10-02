build:
	cat chains/*.yaml > chains.yaml
	go build
	rm -f chains.yaml