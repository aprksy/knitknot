// Import command specs: usage errors, unknown format, and stub dispatch.
package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/spf13/cobra"
)

type stubImporter struct { // ext: import-cmd
	called bool  // ext: import-cmd
	err    error // ext: import-cmd
}

func (s *stubImporter) Import(ctx context.Context, st storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error { // ext: import-cmd
	s.called = true // ext: import-cmd
	return s.err    // ext: import-cmd
}

var _ = Describe("import command", func() { // ext: import-cmd
	var (
		savedFlags struct { // ext: import-cmd
			subgraph string // ext: import-cmd
			file     string // ext: import-cmd
		}
		savedFormat   string                         // ext: import-cmd
		savedRegister func(extension.Registry) error // ext: import-cmd
		tmpInput      string                         // ext: import-cmd
	)

	writeInput := func() string { // ext: import-cmd
		f, err := os.CreateTemp("", "knitknot-import-test-*.dat") // ext: import-cmd
		Expect(err).NotTo(HaveOccurred())                         // ext: import-cmd
		_, err = f.Write([]byte("dummy"))                         // ext: import-cmd
		Expect(err).NotTo(HaveOccurred())                         // ext: import-cmd
		Expect(f.Close()).To(Succeed())                           // ext: import-cmd
		return f.Name()                                           // ext: import-cmd
	}

	BeforeEach(func() { // ext: import-cmd
		savedFlags = globalFlags           // ext: import-cmd
		savedFormat = importFlags.format   // ext: import-cmd
		savedRegister = registerExtensions // ext: import-cmd
		globalFlags.file = ""              // ext: import-cmd
		tmpInput = writeInput()            // ext: import-cmd
	})

	AfterEach(func() { // ext: import-cmd
		globalFlags = savedFlags           // ext: import-cmd
		importFlags.format = savedFormat   // ext: import-cmd
		registerExtensions = savedRegister // ext: import-cmd
		os.Remove(tmpInput)                // ext: import-cmd
	})

	It("errors on unknown format", func() { // ext: import-cmd
		importFlags.format = "nope"                                       // ext: import-cmd
		err := runImport(&cobra.Command{}, []string{tmpInput})            // ext: import-cmd
		Expect(err).To(HaveOccurred())                                    // ext: import-cmd
		Expect(err.Error()).To(ContainSubstring("unknown import format")) // ext: import-cmd
	})

	It("errors with usage when input arg is missing", func() { // ext: import-cmd
		importFlags.format = "stix"                        // ext: import-cmd
		err := runImport(&cobra.Command{}, nil)            // ext: import-cmd
		Expect(err).To(HaveOccurred())                     // ext: import-cmd
		Expect(err.Error()).To(ContainSubstring("usage:")) // ext: import-cmd
	})

	It("errors with usage when format is empty", func() { // ext: import-cmd
		importFlags.format = ""                                // ext: import-cmd
		err := runImport(&cobra.Command{}, []string{tmpInput}) // ext: import-cmd
		Expect(err).To(HaveOccurred())                         // ext: import-cmd
		Expect(err.Error()).To(ContainSubstring("usage:"))     // ext: import-cmd
	})

	It("dispatches to the registered importer end-to-end", func() { // ext: import-cmd
		stub := &stubImporter{}                                 // ext: import-cmd
		registerExtensions = func(r extension.Registry) error { // ext: import-cmd
			Expect(r.RegisterImporter("stub", stub)).To(Succeed()) // ext: import-cmd
			r.Freeze()                                             // ext: import-cmd
			return nil                                             // ext: import-cmd
		}
		globalFlags.file = ""                                                 // ext: import-cmd
		importFlags.format = "stub"                                           // ext: import-cmd
		Expect(runImport(&cobra.Command{}, []string{tmpInput})).To(Succeed()) // ext: import-cmd
		Expect(stub.called).To(BeTrue())                                      // ext: import-cmd
	})

	It("persists to disk when -f is set", func() { // ext: import-cmd
		stub := &stubImporter{}                                 // ext: import-cmd
		registerExtensions = func(r extension.Registry) error { // ext: import-cmd
			Expect(r.RegisterImporter("stub", stub)).To(Succeed()) // ext: import-cmd
			r.Freeze()                                             // ext: import-cmd
			return nil                                             // ext: import-cmd
		}
		gob := filepath.Join(GinkgoT().TempDir(), "out.gob")                  // ext: import-cmd
		globalFlags.file = gob                                                // ext: import-cmd
		importFlags.format = "stub"                                           // ext: import-cmd
		Expect(runImport(&cobra.Command{}, []string{tmpInput})).To(Succeed()) // ext: import-cmd
		Expect(stub.called).To(BeTrue())                                      // ext: import-cmd
		Expect(gob).To(BeAnExistingFile())                                    // ext: import-cmd
	})
})
