package main

import (
	"testing"
	"os"
)

func BenchmarkCrawler(b *testing.B) {
	os.Setenv("WEBSITE", "http://tomblomfield.com")
	main()
}