package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sync command", func() {
	It("returns a usage error when --from is missing", func() {
		c := newSyncCmd()
		c.SetArgs([]string{"--to", "gob:b.gob"})
		err := c.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--from"))
	})

	It("returns a usage error when --to is missing", func() {
		c := newSyncCmd()
		c.SetArgs([]string{"--from", "gob:a.gob"})
		err := c.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--to"))
	})

	It("returns the not-implemented error naming both schemes", func() {
		c := newSyncCmd()
		c.SetArgs([]string{"--from", "gob:a.gob", "--to", "gob:b.gob"})
		err := c.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("sync not implemented"))
		Expect(err.Error()).To(ContainSubstring("gob -> gob"))
	})
})
