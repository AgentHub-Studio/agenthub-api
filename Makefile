.PHONY: compile test package lint tidy

compile:
	./build.sh compile

test:
	./build.sh test

package:
	./build.sh package

lint:
	./build.sh lint

tidy:
	./build.sh tidy
