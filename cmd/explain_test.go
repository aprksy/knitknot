// Locks down H5: printExplain must not panic on non-string args
// (numeric Find label, numeric Has value, non-numeric Limit).
package cmd

import (
	"bytes"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/dsl"
)

// captureStdout redirects os.Stdout (where printExplain writes) into a string.
func captureStdout(f func()) string {
	original := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = original }()

	f()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

var _ = Describe("printExplain H5 regression", func() {
	It("does not panic on Find(5) and prints '???' label", func() {
		ast, err := dsl.NewParser("Find(5)").Parse()
		Expect(err).NotTo(HaveOccurred())

		var output string
		Expect(func() {
			output = captureStdout(func() { printExplain("Find(5)", ast) })
		}).NotTo(Panic())
		Expect(output).To(ContainSubstring("???"))
	})

	It("does not panic on Has with numeric second arg and prints fallback", func() {
		ast, err := dsl.NewParser("Find('User').Has('likes', 5)").Parse()
		Expect(err).NotTo(HaveOccurred())

		var output string
		Expect(func() {
			output = captureStdout(func() { printExplain("Find('User').Has('likes', 5)", ast) })
		}).NotTo(Panic())
		Expect(output).To(ContainSubstring("Has (unrecognized arg)"))
	})

	It("does not panic on Limit with non-numeric arg and prints fallback", func() {
		ast, err := dsl.NewParser("Find('User').Limit('x')").Parse()
		Expect(err).NotTo(HaveOccurred())

		var output string
		Expect(func() {
			output = captureStdout(func() { printExplain("Find('User').Limit('x')", ast) })
		}).NotTo(Panic())
		Expect(output).To(ContainSubstring("Limit (unrecognized arg)"))
	})
})
