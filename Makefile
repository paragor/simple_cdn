build_local:
	goreleaser release --snapshot --clean

push_patch:
	next_tag=$$(cd tools && go run next_tag/main.go patch) && git tag $$next_tag && git push origin tag $$next_tag

push_minor:
	next_tag=$$(cd tools && go run next_tag/main.go minor) && git tag $$next_tag && git push origin tag $$next_tag
