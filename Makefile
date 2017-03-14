
all: test install
	@echo "Done"

install:
	go install github.com/yingshengtech/ffjson

deps:

fmt:
	go fmt github.com/yingshengtech/ffjson/...

cov:
	# TODO: cleanup this make target.
	mkdir -p coverage
	rm -f coverage/*.html
	# gocov test github.com/yingshengtech/ffjson/generator | gocov-html > coverage/generator.html
	# gocov test github.com/yingshengtech/ffjson/inception | gocov-html > coverage/inception.html
	gocov test github.com/yingshengtech/ffjson/fflib/v1 | gocov-html > coverage/fflib.html
	@echo "coverage written"

test-core:
	go test -v github.com/yingshengtech/ffjson/fflib/v1 github.com/yingshengtech/ffjson/generator github.com/yingshengtech/ffjson/inception

test: ffize test-core
	go test -v github.com/yingshengtech/ffjson/tests/...

ffize: install
	ffjson -force-regenerate tests/ff.go
	ffjson -force-regenerate tests/goser/ff/goser.go
	ffjson -force-regenerate tests/go.stripe/ff/customer.go
	ffjson -force-regenerate -reset-fields tests/types/ff/everything.go
	ffjson -force-regenerate tests/number/ff/number.go

bench: ffize all
	go test -v -benchmem -bench MarshalJSON  github.com/yingshengtech/ffjson/tests
	go test -v -benchmem -bench MarshalJSON  github.com/yingshengtech/ffjson/tests/goser github.com/yingshengtech/ffjson/tests/go.stripe
	go test -v -benchmem -bench UnmarshalJSON  github.com/yingshengtech/ffjson/tests/goser github.com/yingshengtech/ffjson/tests/go.stripe

clean:
	go clean -i github.com/yingshengtech/ffjson/...
	rm -rf tests/ff/*_ffjson.go tests/*_ffjson.go tests/ffjson-inception*

.PHONY: deps clean test fmt install all
