package dot_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// exp: dot-tests — Ginkgo suite boot for the DOT exporter.
func TestDot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dot Suite")
}
