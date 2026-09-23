package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolveStoreURI", func() {
	It("defaults to mem when both flags are empty", func() {
		scheme, path, err := ResolveStoreURI("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(scheme).To(Equal("mem"))
		Expect(path).To(Equal(""))
	})

	It("maps -f to gob:<path>", func() {
		scheme, path, err := ResolveStoreURI("", "data.gob")
		Expect(err).NotTo(HaveOccurred())
		Expect(scheme).To(Equal("gob"))
		Expect(path).To(Equal("data.gob"))
	})

	It("parses --store on the first colon", func() {
		scheme, path, err := ResolveStoreURI("gob:data.gob", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(scheme).To(Equal("gob"))
		Expect(path).To(Equal("data.gob"))
	})

	It("prefers --store over -f", func() {
		scheme, path, err := ResolveStoreURI("gob:a.gob", "b.gob")
		Expect(err).NotTo(HaveOccurred())
		Expect(scheme).To(Equal("gob"))
		Expect(path).To(Equal("a.gob"))
	})

	It("errors on --store without a scheme", func() {
		_, _, err := ResolveStoreURI("noscheme", "")
		Expect(err).To(HaveOccurred())
		_, _, err = ResolveStoreURI(":path", "")
		Expect(err).To(HaveOccurred())
	})
})
