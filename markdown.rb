all
rule 'MD013', :line_length => 500
rule 'MD024', :allow_different_nesting => true
rule 'MD029', :style => 'ordered'

exclude_rule 'MD014'
exclude_rule 'MD025'
exclude_rule 'MD033'
exclude_rule 'MD034' # doesn't work for urls in code blocks
exclude_rule 'MD036' # this will prevent __! text __
