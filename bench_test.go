package main

import (
	"testing"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

func BenchmarkAboutCached(b *testing.B) {
	m := aboutModel()
	th := views.Themes[m.themeIdx]
	m.renderBody(th) // warm
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderBody(th)
	}
}

func BenchmarkAboutUncached(b *testing.B) {
	m := aboutModel()
	th := views.Themes[m.themeIdx]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderBodyUncached(th)
	}
}
