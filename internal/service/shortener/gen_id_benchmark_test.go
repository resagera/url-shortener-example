package service

import (
	"testing"
	"time"
)

func BenchmarkGenerateCodeThroughput(b *testing.B) {
	b.ReportAllocs()

	start := time.Now()
	for i := 0; i < b.N; i++ {
		_ = generateCode(10)
	}
	elapsed := time.Since(start)

	// сколько кодов в секунду
	codesPerSec := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(codesPerSec, "id/s")
}
