

markdown-lint:
	# https://github.com/markdownlint/markdownlint/blob/master/docs/RULES.md
	# https://github.com/markdownlint/markdownlint/blob/master/lib/mdl/rules.rb
	HOME=/workdir mdl -s markdown.rb .
