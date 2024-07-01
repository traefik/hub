.PHONY: clean lint test

default: clean lint test

lint:
	golangci-lint run

clean:
	go clean -testcache
	rm -rf cover.out

test: clean
	go test -v -race -cover ./...

test-apigw: clean
	go test -run TestAPIGatewayTestSuite github.com/traefik/hub/tests/apigateway

test-apim: clean
	go test -run TestAPIManagementTestSuite github.com/traefik/hub/tests/apimanagement

test-walkthrough: clean
	go test -run TestWalkthroughTestSuite github.com/traefik/hub/tests/walkthrough

test-testhelpers: clean
	go test github.com/traefik/hub/tests/testhelpers
