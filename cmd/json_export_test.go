// Locks down M7: runExport with --format json returns a "not implemented" error.
// Approach: globalFlags.file is left empty so LoadGraph returns a fresh
// in-memory engine without touching disk; the format switch is reached and
// the json branch returns before any output is written.
package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("export --format json (M7)", func() {
	var (
		origFile, origSubgraph string
		origFormat, origOutput string
	)

	BeforeEach(func() {
		origFile, origSubgraph = globalFlags.file, globalFlags.subgraph
		origFormat, origOutput = exportFlags.format, exportFlags.output
		globalFlags.file = ""
		globalFlags.subgraph = ""
		exportFlags.format = "json"
		exportFlags.output = ""
	})

	AfterEach(func() {
		globalFlags.file, globalFlags.subgraph = origFile, origSubgraph
		exportFlags.format, exportFlags.output = origFormat, origOutput
	})

	It("returns a not-implemented error", func() {
		err := runExport(nil, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not implemented"))
	})
})
