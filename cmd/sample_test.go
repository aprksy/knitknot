// Locks down L6: runGenerateSample without -f returns the usage error.
package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("generate-sample without -f (L6)", func() {
	var origFile string

	BeforeEach(func() {
		origFile = globalFlags.file
		globalFlags.file = ""
	})

	AfterEach(func() {
		globalFlags.file = origFile
	})

	It("returns the usage error", func() {
		err := runGenerateSample(nil, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("usage: knitknot generate-sample -f <file.gob>"))
	})
})
