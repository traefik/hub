.PHONY: clean lint test

default: clean lint test

lint:
	golangci-lint run

clean:
	rm -rf cover.out

test: clean
	go test -v -race -cover ./...

test-apigw:
	go test -run TestAPIGatewayTestSuite github.com/traefik/hub/tests/apigateway

test-apim:
	go test -run TestAPIManagementTestSuite github.com/traefik/hub/tests/apimanagement

test-walkthrough:
	go test -run TestWalkthroughTestSuite github.com/traefik/hub/tests/walkthrough
