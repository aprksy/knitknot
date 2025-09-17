package inmem_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInmem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Inmem Suite")
}
