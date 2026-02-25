
.PHONY: clean build test native run

run:
	@mvn quarkus:dev

clean:
	@mvn clean

install:
	@mvn clean install -DskipTests

test:
	@mvn test

build: clean
	@mvn install -DskipTests -Dnative
