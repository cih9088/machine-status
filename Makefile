NAME = cih9088/machine-status
VERSION = 0.3.4

.PHONY: build build-nocache test tag-latest push push-latest release git-tag-version

build:
	docker build -t $(NAME):$(VERSION) --rm .

build-nocache:
	docker build -t $(NAME):$(VERSION) --no-cache --rm .

tag:
	docker tag $(NAME):$(VERSION) $(NAME):$(VERSION)

tag-latest:
	docker tag $(NAME):$(VERSION) $(NAME):latest

push:
	docker push $(NAME):$(VERSION)

push-latest:
	docker push $(NAME):latest

release: build tag-latest push push-latest

git-tag-version: release
	git tag -a v$(VERSION) -m "v$(VERSION)"
	git push origin v$(VERSION)
