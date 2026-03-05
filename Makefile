BINARY := homedash
SRC := ./cmd/homedash

.PHONY: build run clean

build:
	go build -o $(BINARY) $(SRC)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
