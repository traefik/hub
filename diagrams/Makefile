.PHONY: build diagram clean

DIST_DIR ?= $(CURDIR)/dist
.DEFAULT_GOAL := diagram

help: ## This help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Creates the binary.
	@go build

diagram: clean ## Creates the diagram.
	@./diagrams
	@cd $(DIST_DIR)/overview && dot -Tpng overview.dot > ../overview.png
	@cd $(DIST_DIR)/microservices && circo -Tpng microservices.dot > ../microservices.png

clean: ## Deletes the old diagram.
	@rm -rf $(DIST_DIR)
	@mkdir $(DIST_DIR)
