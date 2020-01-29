package sshkey

import (
	"testing"
)

func BenchmarkGenerate_4096(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Generate(4096)
	}
}

func BenchmarkGenerate_3072(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Generate(3072)
	}
}

func BenchmarkGenerate_1024(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Generate(1024)
	}
}

func BenchmarkFactory_3072(b *testing.B) {
	factory := NewFactory(10, 3072)
	go factory.Run()
	for i := 0; i < b.N; i++ {
		_ = factory.Get()
	}
}
